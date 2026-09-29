package lead

import "testing"

func TestDeleteRemovesLeadAndHistory(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	a, _ := repo.Create(Input{Name: "A", Email: "a@example.com"})
	b, _ := repo.Create(Input{Name: "B", Email: "b@example.com"})
	c, _ := repo.Create(Input{Name: "C", Email: "c@example.com"})
	if err := repo.RecordSend(SentMessage{LeadID: a, Mode: "initial", Subject: "Hi", Body: "Hello"}); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.Delete(a); err != nil || n != 1 {
		t.Fatalf("delete one: n=%d err=%v", n, err)
	}
	if events, _ := repo.Events(a); len(events) != 0 {
		t.Errorf("expected history to be removed, got %d events", len(events))
	}
	if n, err := repo.Delete(b, c, 999); err != nil || n != 2 {
		t.Fatalf("bulk delete: n=%d err=%v", n, err)
	}
	if leads, _ := repo.List(ListFilter{}); len(leads) != 0 {
		t.Errorf("expected no leads, got %d", len(leads))
	}
	// A deleted email can be added again.
	if _, err := repo.Create(Input{Email: "a@example.com"}); err != nil {
		t.Errorf("re-create after delete: %v", err)
	}
}

func TestImportReportsEveryRow(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	repo.Create(Input{Email: "exists@example.com"})
	results, err := repo.Import([]Input{
		{Name: "Ravi", Organization: "Acme", Email: "Ravi@Example.com", Designation: "CEO", Address: "Pune"},
		{Name: "Dup", Email: "exists@example.com"},
		{Name: "Bad", Email: "not-an-email"},
		{Name: "Nobody"},
		{Name: "Same file dup", Email: "ravi@example.com"},
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"added", "duplicate", "invalid", "empty", "duplicate"}
	for i, r := range results {
		if r.Status != want[i] || r.Row != i+2 {
			t.Errorf("row %d: got %s (row %d), want %s", i, r.Status, r.Row, want[i])
		}
	}
	l, _ := repo.Get(results[0].ID)
	if l.Email != "ravi@example.com" || l.Organization != "Acme" || l.Designation != "CEO" || l.Address != "Pune" {
		t.Errorf("unexpected stored lead: %+v", l)
	}
}

func TestUnsubscribeExcludesFromCampaigns(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	id, _ := repo.Create(Input{Email: "u@example.com"})
	token, err := repo.EnsureUnsubToken(id)
	if err != nil || len(token) != 32 {
		t.Fatalf("token=%q err=%v", token, err)
	}
	again, _ := repo.EnsureUnsubToken(id)
	if again != token {
		t.Error("expected a stable token")
	}
	if ok, err := repo.Unsubscribe(token); !ok || err != nil {
		t.Fatalf("unsubscribe ok=%v err=%v", ok, err)
	}
	if ok, _ := repo.Unsubscribe("bogus-token-that-is-long"); ok {
		t.Error("expected unknown token to fail")
	}
	if targets, _ := repo.CampaignTargets(Campaign{Mode: "initial"}); len(targets) != 0 {
		t.Errorf("unsubscribed lead should not be targeted")
	}
}

func TestConversationsAndWebhookMatchingByEmailID(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	a, _ := repo.Create(Input{Name: "Asha", Email: "asha@example.com"})
	repo.Create(Input{Name: "Never emailed", Email: "n@example.com"})
	repo.RecordSend(SentMessage{LeadID: a, Mode: "initial", Subject: "Hello Asha", Body: "Hi Asha", To: "asha@example.com", ResendID: "em_1"})
	// Delivery event with a different "to" casing still matches by the Resend id.
	if ok, err := repo.ProcessWebhook("r1", WebhookEvent{Type: "email.delivered", EmailID: "em_1", Recipient: "other@example.com"}); !ok || err != nil {
		t.Fatalf("delivered ok=%v err=%v", ok, err)
	}
	repo.ProcessWebhook("r2", WebhookEvent{Type: "email.received", EmailID: "in_1", Sender: "asha@example.com", Recipient: "reply@us.com", Subject: "Re: Hello Asha", Content: "Sounds good"})
	all, err := repo.Conversations(false, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("expected 1 conversation, got %d (%v)", len(all), err)
	}
	c := all[0]
	if c.Lead.ID != a || c.LastKind != "reply" || c.LastSnippet != "Sounds good" || c.Messages != 2 || c.Replies != 1 {
		t.Errorf("unexpected conversation: %+v", c)
	}
	replied, _ := repo.Conversations(true, "asha")
	if len(replied) != 1 {
		t.Errorf("expected replied filter to find Asha")
	}
	events, _ := repo.Events(a)
	kinds := []string{}
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	if len(events) != 3 || events[0].Kind != "initial" || events[2].Kind != "reply" || events[2].From != "asha@example.com" {
		t.Errorf("unexpected thread: %v", kinds)
	}
}

func TestRender(t *testing.T) {
	l := Lead{Name: "Dr. Anil Kumar", Organization: "City Hospital", Designation: "", Address: "Delhi"}
	got := Render("Hi {{first_name}}, {{ organization }} team ({{designation|leadership}}) in {{address}}. {{company}} {{unknown}}", l)
	want := "Hi Anil, City Hospital team (leadership) in Delhi. City Hospital {{unknown}}"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if got := Render("Hi {{name}},", Lead{}); got != "Hi," {
		t.Errorf("empty name: got %q", got)
	}
	if u := UnknownPlaceholders("{{name}} {{foo}}", "{{bar|x}}"); len(u) != 2 || u[0] != "bar" || u[1] != "foo" {
		t.Errorf("unknown: %v", u)
	}
	if html := PlainToHTML("Hello <b>\n\nVisit https://x.com."); html != `<div style="font-family:Arial,Helvetica,sans-serif;font-size:14px;line-height:1.55;color:#222"><p style="margin:0 0 14px">Hello &lt;b&gt;</p><p style="margin:0 0 14px">Visit <a href="https://x.com">https://x.com</a>.</p></div>` {
		t.Errorf("html: %s", html)
	}
}

func TestMapHeaders(t *testing.T) {
	headers := []string{"\xef\xbb\xbfS.No", "Company Name", "Contact Person", "Email ID", "Mobile No.", "Job Title", "City", "Industry", "Revenue"}
	m := MapHeaders(headers)
	want := map[string]int{"organization": 1, "name": 2, "email": 3, "phone": 4, "designation": 5, "address": 6, "segment": 7}
	for field, index := range want {
		if m[field] != index {
			t.Errorf("%s: got column %d, want %d", field, m[field], index)
		}
	}
	if len(m) != len(want) {
		t.Errorf("unexpected extra mapping: %v", m)
	}
	first, last := FirstLastColumns([]string{"First Name", "Last Name", "Email"})
	if first != 0 || last != 1 {
		t.Errorf("first/last: %d %d", first, last)
	}
	if CleanEmail(" Mailto:A@B.com; c@d.com ") != "a@b.com" || CleanPhone("9876543210.0") != "9876543210" || CleanPhone("'+91 98765") != "+91 98765" {
		t.Error("cleaning failed")
	}
}
