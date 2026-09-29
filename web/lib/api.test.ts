import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { api } from './api.ts';

test('Frontend API client methods', async (t) => {
  let lastRequest: { method?: string; url?: string; body?: any; headers?: http.IncomingHttpHeaders } = {};

  const server = http.createServer((req, res) => {
    let body = '';
    req.on('data', chunk => { body += chunk; });
    req.on('end', () => {
      lastRequest = {
        method: req.method,
        url: req.url,
        body: body ? JSON.parse(body) : null,
        headers: req.headers,
      };

      res.setHeader('Content-Type', 'application/json');

      if (req.url?.startsWith('/api/leads?q=')) {
        res.writeHead(200);
        res.end(JSON.stringify([{ id: 1, name: 'Lead 1' }]));
      } else if (req.url === '/api/dashboard') {
        res.writeHead(200);
        res.end(JSON.stringify({ total: 10, emailed: 5, opened: 2, replied: 1, invalid: 0 }));
      } else if (req.url === '/api/leads' && req.method === 'POST') {
        res.writeHead(201);
        res.end(JSON.stringify({ id: 42, name: 'Bob' }));
      } else if (req.url === '/api/import' && req.method === 'POST') {
        res.writeHead(200);
        res.end(JSON.stringify({ imported: 5, rows: 6, skipped: 1, duplicates: 0, empty: 1, invalid: 0 }));
      } else if (req.url === '/api/automation/send' && req.method === 'POST') {
        res.writeHead(202);
        res.end(JSON.stringify({ id: 7, status: 'running', total: 3, sent: 0, failed: 0 }));
      } else if (req.url === '/api/leads/5' && req.method === 'DELETE') {
        res.writeHead(200);
        res.end(JSON.stringify({ deleted: 1 }));
      } else if (req.url === '/api/leads/delete' && req.method === 'POST') {
        res.writeHead(200);
        res.end(JSON.stringify({ deleted: 2 }));
      } else if (req.url?.startsWith('/api/conversations')) {
        res.writeHead(200);
        res.end(JSON.stringify([]));
      } else if (req.url === '/api/leads/1/events') {
        res.writeHead(200);
        res.end(JSON.stringify([{ id: 1, kind: 'initial' }]));
      } else if (req.url === '/api/leads/1/reply' && req.method === 'POST') {
        res.writeHead(200);
        res.end(JSON.stringify({ ok: true }));
      } else if (req.url === '/api/leads/1/invalid' && req.method === 'PATCH') {
        res.writeHead(200);
        res.end(JSON.stringify({ ok: true }));
      } else if (req.url === '/api/error') {
        res.writeHead(400);
        res.end(JSON.stringify({ error: 'Custom error message' }));
      } else {
        res.writeHead(404);
        res.end(JSON.stringify({ error: 'not found' }));
      }
    });
  });

  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', () => resolve()));
  const address = server.address() as { port: number };
  process.env.NEXT_PUBLIC_API_URL = `http://127.0.0.1:${address.port}`;

  t.after(() => {
    server.close();
  });

  await t.test('listLeads sends GET with encoded query', async () => {
    const leads = await api.listLeads('John & Jane');
    assert.equal(lastRequest.method, 'GET');
    assert.equal(lastRequest.url, '/api/leads?q=John%20%26%20Jane&segment=');
    assert.deepEqual(leads, [{ id: 1, name: 'Lead 1' }]);
  });

  await t.test('dashboard sends GET to /api/dashboard', async () => {
    const stats = await api.dashboard();
    assert.equal(lastRequest.method, 'GET');
    assert.equal(lastRequest.url, '/api/dashboard');
    assert.equal(stats.total, 10);
  });

  await t.test('createLead sends POST with JSON payload', async () => {
    const res = await api.createLead({ name: 'Bob', email: 'bob@example.com' });
    assert.equal(lastRequest.method, 'POST');
    assert.equal(lastRequest.url, '/api/leads');
    assert.deepEqual(lastRequest.body, { name: 'Bob', email: 'bob@example.com' });
    assert.equal(res.id, 42);
  });

  await t.test('importLeads sends POST to /api/import', async () => {
    const res = await api.importLeads({ csv: 'name,email\nTest,test@test.com', mapping: { address: '' }, segment: 'Schools' });
    assert.equal(lastRequest.method, 'POST');
    assert.equal(lastRequest.url, '/api/import');
    assert.deepEqual(lastRequest.body, { csv: 'name,email\nTest,test@test.com', mapping: { address: '' }, segment: 'Schools' });
    assert.equal(res.imported, 5);
  });

  await t.test('sendCampaign sends POST to /api/automation/send', async () => {
    const res = await api.sendCampaign({ subject: 'Sub', body: 'Bod', mode: 'initial', segment: '', before: '', noFollowup: false, maxFollowups: 0, trackOpens: false, includeUnsubscribe: true });
    assert.equal(lastRequest.method, 'POST');
    assert.equal(lastRequest.url, '/api/automation/send');
    assert.equal(res.id, 7);
    assert.equal(res.status, 'running');
  });

  await t.test('deleteLead and deleteLeads', async () => {
    assert.equal((await api.deleteLead(5)).deleted, 1);
    assert.equal(lastRequest.method, 'DELETE');
    assert.equal((await api.deleteLeads([1, 2])).deleted, 2);
    assert.deepEqual(lastRequest.body, { ids: [1, 2] });
  });

  await t.test('conversations passes filter', async () => {
    await api.conversations('replied', 'acme');
    assert.equal(lastRequest.url, '/api/conversations?filter=replied&q=acme');
  });

  await t.test('errors surface the API message', async () => {
    await assert.rejects(() => api.getLead(999), /not found/);
  });

  await t.test('events sends GET to /api/leads/:id/events', async () => {
    const res = await api.events(1);
    assert.equal(lastRequest.method, 'GET');
    assert.equal(lastRequest.url, '/api/leads/1/events');
    assert.equal(res.length, 1);
  });

  await t.test('recordReply sends POST to /api/leads/:id/reply', async () => {
    const res = await api.recordReply(1, 'Interested');
    assert.equal(lastRequest.method, 'POST');
    assert.equal(lastRequest.url, '/api/leads/1/reply');
    assert.deepEqual(lastRequest.body, { content: 'Interested' });
    assert.equal(res.ok, true);
  });

  await t.test('markInvalid sends PATCH to /api/leads/:id/invalid', async () => {
    const res = await api.markInvalid(1);
    assert.equal(lastRequest.method, 'PATCH');
    assert.equal(lastRequest.url, '/api/leads/1/invalid');
    assert.equal(res.ok, true);
  });
});
