package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"automationtool/api/internal/database"
	"automationtool/api/internal/lead"
	"automationtool/api/internal/mailer"
)

type mockMailer struct {
	mu      sync.Mutex
	sendErr error
	sent    []mailer.Message
}

func (m *mockMailer) Send(ctx context.Context, message mailer.Message) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return "", m.sendErr
	}
	m.sent = append(m.sent, message)
	return fmt.Sprintf("em_%d", len(m.sent)), nil
}

type mockInboundReader struct {
	text string
	err  error
}

func (m *mockInboundReader) ReceivedText(ctx context.Context, emailID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.text, nil
}

func setupTestServer(t *testing.T) (*Server, *lead.Repository, *mockMailer, *mockInboundReader, string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "httpapi_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	repo := lead.NewRepository(db)
	mailer := &mockMailer{}
	inbound := &mockInboundReader{text: "Inbound reply content"}
	rawKey := []byte("12345678901234567890123456789012")
	secret := "whsec_" + base64.StdEncoding.EncodeToString(rawKey)
	server := New(repo, mailer, inbound, secret, "http://localhost:3000")
	server.Configure(SendSettings{PublicURL: "https://api.example.com"})

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}
	return server, repo, mailer, inbound, secret, cleanup
}

func TestHealthAndCORS(t *testing.T) {
	server, _, _, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := server.Routes()

	// 1. Health check
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
	var res map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || res["status"] != "ok" {
		t.Errorf("unexpected health response: %v", rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// 2. CORS OPTIONS preflight
	optReq := httptest.NewRequest(http.MethodOptions, "/api/leads", nil)
	optRec := httptest.NewRecorder()
	handler.ServeHTTP(optRec, optReq)
	if optRec.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for OPTIONS, got %d", optRec.Code)
	}

	// 3. Root endpoint check
	rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rootRec := httptest.NewRecorder()
	handler.ServeHTTP(rootRec, rootReq)
	if rootRec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for root endpoint, got %d", rootRec.Code)
	}
	var rootData map[string]any
	if err := json.Unmarshal(rootRec.Body.Bytes(), &rootData); err != nil || rootData["status"] != "running" {
		t.Errorf("unexpected root response: %s", rootRec.Body.String())
	}

	// 4. Missing path returns 404
	missingReq := httptest.NewRequest(http.MethodGet, "/unknown-page", nil)
	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", missingRec.Code)
	}
}

func TestLeadsHandlers(t *testing.T) {
	server, repo, _, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := server.Routes()

	// 1. Create Lead - Bad JSON
	req := httptest.NewRequest(http.MethodPost, "/api/leads", bytes.NewBufferString("invalid json"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rec.Code)
	}

	// 2. Create Lead - Missing email and phone
	emptyPayload, _ := json.Marshal(lead.Input{Name: "No Contact"})
	req = httptest.NewRequest(http.MethodPost, "/api/leads", bytes.NewBuffer(emptyPayload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when missing email and phone, got %d", rec.Code)
	}

	// 2b. Create Lead - Invalid email address
	badEmailPayload, _ := json.Marshal(lead.Input{Email: "invalid-email"})
	req = httptest.NewRequest(http.MethodPost, "/api/leads", bytes.NewBuffer(badEmailPayload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid email address, got %d", rec.Code)
	}

	// 3. Create Lead - Valid
	validPayload, _ := json.Marshal(lead.Input{
		Name:         "Test Lead",
		Email:        "test@example.com",
		Phone:        "555-0199",
		Organization: "Finance Co",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/leads", bytes.NewBuffer(validPayload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	var created lead.Lead
	json.Unmarshal(rec.Body.Bytes(), &created)
	leadID := created.ID
	if created.Organization != "Finance Co" || created.Email != "test@example.com" {
		t.Errorf("expected the created lead in the response, got %+v", created)
	}
	if leadID <= 0 {
		t.Fatalf("expected valid lead ID, got %d", leadID)
	}

	// 4. Create Lead - Duplicate (conflict)
	req = httptest.NewRequest(http.MethodPost, "/api/leads", bytes.NewBuffer(validPayload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict on duplicate, got %d", rec.Code)
	}

	// 5. List Leads
	req = httptest.NewRequest(http.MethodGet, "/api/leads?q=Test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK on list, got %d", rec.Code)
	}
	var list []lead.Lead
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != leadID {
		t.Errorf("expected 1 lead matching query, got %v", list)
	}

	// 6. Lead Detail - Bad ID
	req = httptest.NewRequest(http.MethodGet, "/api/leads/not-an-id/events", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad lead ID, got %d", rec.Code)
	}

	// 7. Lead Detail - Unknown Action
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/leads/%d/unknown", leadID), nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown action, got %d", rec.Code)
	}

	// 8. Lead Detail - Record Reply (Bad JSON)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/leads/%d/reply", leadID), bytes.NewBufferString("{bad"))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on bad JSON for reply, got %d", rec.Code)
	}

	// 9. Lead Detail - Record Reply (Valid)
	replyPayload, _ := json.Marshal(map[string]string{"content": "I am interested"})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/leads/%d/reply", leadID), bytes.NewBuffer(replyPayload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK on reply, got %d", rec.Code)
	}

	// Verify reply recorded in lead
	leads, _ := repo.List(lead.ListFilter{})
	if !leads[0].Replied {
		t.Errorf("expected lead to be marked replied")
	}

	// 10. Lead Detail - Events
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/leads/%d/events", leadID), nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for events, got %d", rec.Code)
	}
	var events []lead.Event
	json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 1 || events[0].Kind != "reply" {
		t.Errorf("expected 1 reply event, got %v", events)
	}

	// 11. Lead Detail - Mark Invalid
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/leads/%d/invalid", leadID), nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for mark invalid, got %d", rec.Code)
	}
	leads, _ = repo.List(lead.ListFilter{})
	if !leads[0].IsInvalid {
		t.Errorf("expected lead to be marked invalid")
	}
}

func TestImportHandler(t *testing.T) {
	server, repo, _, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := server.Routes()

	// 1. Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/import", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on bad JSON, got %d", rec.Code)
	}

	// 2. CSV with insufficient rows
	payload, _ := json.Marshal(map[string]string{"csv": "name,email"})
	req = httptest.NewRequest(http.MethodPost, "/api/import", bytes.NewBuffer(payload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on <2 rows CSV, got %d", rec.Code)
	}

	// 3. CSV without email or phone column
	payload, _ = json.Marshal(map[string]string{"csv": "name,industry\nBob,Sales"})
	req = httptest.NewRequest(http.MethodPost, "/api/import", bytes.NewBuffer(payload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on CSV without email/phone, got %d", rec.Code)
	}

	// 4. Valid CSV import
	csvData := "name,email,phone,category,subcategory\n" +
		"User One,user1@example.com,111-222,Tech,AI\n" +
		"User Two,user2@example.com,,Tech,Cloud\n" +
		"Empty Row,,,, \n" +
		"Duplicate User,user1@example.com,999-999,Tech,AI\n"
	payload, _ = json.Marshal(map[string]string{"csv": csvData})
	req = httptest.NewRequest(http.MethodPost, "/api/import", bytes.NewBuffer(payload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on CSV import, got %d: %s", rec.Code, rec.Body.String())
	}
	var importRes struct {
		Imported, Duplicates, Empty, Rows int
		Results                           []lead.ImportResult
	}
	json.Unmarshal(rec.Body.Bytes(), &importRes)
	// User1 and User2 imported (2), Empty Row skipped, Duplicate User skipped
	if importRes.Imported != 2 || importRes.Duplicates != 1 || importRes.Empty != 1 || importRes.Rows != 4 || len(importRes.Results) != 4 {
		t.Errorf("unexpected import summary: %+v", importRes)
	}

	// 5. Test Google Sheets CSV import via mock HTTP server
	mockSheetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("name,email\nSheet User,sheet@example.com\n"))
	}))
	defer mockSheetServer.Close()

	sheetPayload, _ := json.Marshal(map[string]string{"url": mockSheetServer.URL})
	// But note: googleSheetsCSVURL requires HTTPS!
	// Let's test that non-HTTPS returns error:
	req = httptest.NewRequest(http.MethodPost, "/api/import", bytes.NewBuffer(sheetPayload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-HTTPS sheet URL, got %d", rec.Code)
	}

	leads, _ := repo.List(lead.ListFilter{})
	if len(leads) != 2 {
		t.Errorf("expected 2 leads in database, got %d", len(leads))
	}
}

func TestGoogleSheetsCSVURL(t *testing.T) {
	// 1. Invalid URL
	if _, err := googleSheetsCSVURL("http://not-https.com"); err == nil {
		t.Errorf("expected error for non-https URL")
	}

	// 2. Standard Google Sheets URL
	input := "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit?gid=123#gid=123"
	got, err := googleSheetsCSVURL(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/export?format=csv&gid=123"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}

	// 3. Already published URL (/d/e/...)
	published := "https://docs.google.com/spreadsheets/d/e/2PACX-1vR.../pub?output=csv"
	got, err = googleSheetsCSVURL(published)
	if err != nil || got != published {
		t.Errorf("expected published URL to be returned as-is, got %s, err: %v", got, err)
	}
}

func TestAutomationHandler(t *testing.T) {
	server, repo, mailer, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := server.Routes()

	// 1. Bad JSON
	req := httptest.NewRequest(http.MethodPost, "/api/automation/send", bytes.NewBufferString("{bad"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on bad JSON, got %d", rec.Code)
	}

	// 2. Missing subject or body
	payload, _ := json.Marshal(lead.Campaign{Subject: "Only Subject"})
	req = httptest.NewRequest(http.MethodPost, "/api/automation/send", bytes.NewBuffer(payload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on missing body, got %d", rec.Code)
	}

	// 3. No eligible leads (returns 400)
	payload, _ = json.Marshal(lead.Campaign{Subject: "Hello", Body: "World", Mode: "initial"})
	req = httptest.NewRequest(http.MethodPost, "/api/automation/send", bytes.NewBuffer(payload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when no eligible leads, got %d", rec.Code)
	}

	// Create 2 leads
	repo.Create(lead.Input{Name: "Lead 1", Email: "lead1@example.com"})
	repo.Create(lead.Input{Name: "Lead 2", Email: "lead2@example.com"})

	// 4. Send successful campaign (runs in the background)
	req = httptest.NewRequest(http.MethodPost, "/api/automation/send", bytes.NewBuffer(payload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on campaign send, got %d: %s", rec.Code, rec.Body.String())
	}
	var run lead.CampaignRun
	json.Unmarshal(rec.Body.Bytes(), &run)
	server.Wait()
	run, _ = repo.CampaignRun(run.ID)
	if run.Status != "completed" || run.Sent != 2 || run.Total != 2 {
		t.Errorf("unexpected run: %+v", run)
	}
	if len(mailer.sent) != 2 {
		t.Errorf("expected mailer to have sent 2 emails, got %d", len(mailer.sent))
	}

	// 5. Error simulation in mailer: the run finishes as failed and the failure is logged per lead
	id3, _ := repo.Create(lead.Input{Name: "Lead 3", Email: "lead3@example.com"})
	mailer.sendErr = errors.New("smtp connection failed")
	req = httptest.NewRequest(http.MethodPost, "/api/automation/send", bytes.NewBuffer(payload))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &run)
	server.Wait()
	run, _ = repo.CampaignRun(run.ID)
	if run.Status != "failed" || run.Failed != 1 || run.LastError == "" {
		t.Errorf("expected failed run, got %+v", run)
	}
	events, _ := repo.Events(id3)
	if len(events) != 1 || events[0].Kind != "failed" {
		t.Errorf("expected a failed event, got %+v", events)
	}
}

func TestDashboardAndTrackingHandlers(t *testing.T) {
	server, repo, _, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := server.Routes()

	id, _ := repo.Create(lead.Input{Name: "Lead", Email: "lead@example.com"})

	// 1. Dashboard
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for dashboard, got %d", rec.Code)
	}
	var stats lead.Stats
	json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.Total != 1 {
		t.Errorf("expected total 1, got %d", stats.Total)
	}

	// 2. Tracking pixel
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/track/open/%d", id), nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for tracking, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "image/gif" {
		t.Errorf("expected image/gif, got %s", rec.Header().Get("Content-Type"))
	}
	if !bytes.Equal(rec.Body.Bytes(), transparentGIF) {
		t.Errorf("expected transparent GIF bytes")
	}

	// Verify lead was marked opened
	leads, _ := repo.List(lead.ListFilter{})
	if !leads[0].IsOpened {
		t.Errorf("expected lead to be marked opened via tracking pixel")
	}
}

func TestResendWebhookHandler(t *testing.T) {
	server, repo, _, _, secret, cleanup := setupTestServer(t)
	defer cleanup()
	handler := server.Routes()

	repo.Create(lead.Input{Name: "Receiver", Email: "receiver@example.com"})

	rawKey := []byte("12345678901234567890123456789012")

	signRequest := func(body []byte) (string, string, string) {
		msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, rawKey)
		mac.Write([]byte(msgID + "." + ts + "."))
		mac.Write(body)
		sig := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
		return msgID, ts, sig
	}

	// 1. Method not allowed (GET)
	req := httptest.NewRequest(http.MethodGet, "/webhooks/resend", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", rec.Code)
	}

	// 2. Unauthorized (no Svix headers)
	req = httptest.NewRequest(http.MethodPost, "/webhooks/resend", bytes.NewBufferString("{}"))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 3. Unhandled event type -> 204 No Content
	unhandledBody := []byte(`{"type":"email.complained"}`)
	msgID, ts, sig := signRequest(unhandledBody)
	req = httptest.NewRequest(http.MethodPost, "/webhooks/resend", bytes.NewBuffer(unhandledBody))
	req.Header.Set("svix-id", msgID)
	req.Header.Set("svix-timestamp", ts)
	req.Header.Set("svix-signature", sig)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for unhandled event, got %d", rec.Code)
	}

	// 4. Handled event (email.delivered)
	deliveredBody := []byte(`{"type":"email.delivered","data":{"to":["receiver@example.com"],"subject":"Hello"}}`)
	msgID, ts, sig = signRequest(deliveredBody)
	req = httptest.NewRequest(http.MethodPost, "/webhooks/resend", bytes.NewBuffer(deliveredBody))
	req.Header.Set("svix-id", msgID)
	req.Header.Set("svix-timestamp", ts)
	req.Header.Set("svix-signature", sig)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for email.delivered, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Handled event (email.received)
	receivedBody := []byte(`{"type":"email.received","data":{"from":"receiver@example.com","email_id":"email_rec_123","subject":"Re: Hello"}}`)
	msgID, ts, sig = signRequest(receivedBody)
	req = httptest.NewRequest(http.MethodPost, "/webhooks/resend", bytes.NewBuffer(receivedBody))
	req.Header.Set("svix-id", msgID)
	req.Header.Set("svix-timestamp", ts)
	req.Header.Set("svix-signature", sig)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for email.received, got %d: %s", rec.Code, rec.Body.String())
	}
	leads, _ := repo.List(lead.ListFilter{})
	if !leads[0].Replied {
		t.Errorf("expected lead to be marked replied on email.received webhook")
	}
	_ = secret
}
