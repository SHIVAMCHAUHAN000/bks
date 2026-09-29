package lead

import (
	"automationtool/api/internal/database"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) (*Repository, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lead_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	repo := NewRepository(db)
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}
	return repo, cleanup
}

func TestLeadCreateAndList(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create valid lead
	id1, err := repo.Create(Input{
		Name:         "Alice Doe",
		Email:        " Alice@example.COM ",
		Phone:        " 1234567890 ",
		Segment:      "SaaS",
		Organization: "Acme",
	})
	if err != nil {
		t.Fatalf("failed to create lead 1: %v", err)
	}
	if id1 <= 0 {
		t.Fatalf("expected valid id, got %d", id1)
	}

	// 2. Duplicate email should fail
	_, err = repo.Create(Input{
		Name:  "Alice Clone",
		Email: "alice@example.com",
	})
	if err == nil {
		t.Fatalf("expected duplicate email error, got nil")
	}

	// 3. Duplicate phone should fail
	_, err = repo.Create(Input{
		Name:  "Bob",
		Phone: "1234567890",
	})
	if err == nil {
		t.Fatalf("expected duplicate phone error, got nil")
	}

	// 4. Invalid email address should return ErrInvalidEmail
	_, err = repo.Create(Input{
		Name:    "Bad Email Lead",
		Email:   "invalid-email-address",
		Segment: "Retail",
	})
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}

	// 5. Empty emails / empty phones should not conflict with each other
	id3, err := repo.Create(Input{
		Name:    "Empty Email 1",
		Phone:   "111111",
		Segment: "Tech",
	})
	if err != nil {
		t.Fatalf("failed to create lead with empty email 1: %v", err)
	}

	id4, err := repo.Create(Input{
		Name:    "Empty Email 2",
		Phone:   "222222",
		Segment: "Tech",
	})
	if err != nil {
		t.Fatalf("failed to create lead with empty email 2: %v", err)
	}
	if id3 == 0 || id4 == 0 {
		t.Fatalf("unexpected ids for empty email leads")
	}

	// List all (id1, id3, id4)
	allLeads, err := repo.List(ListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(allLeads) != 3 {
		t.Fatalf("expected 3 leads, got %d", len(allLeads))
	}

	// Verify normalization
	var alice Lead
	for _, l := range allLeads {
		if l.ID == id1 {
			alice = l
		}
	}
	if alice.Email != "alice@example.com" {
		t.Errorf("expected trimmed lowercase email, got %s", alice.Email)
	}
	if alice.Phone != "1234567890" {
		t.Errorf("expected trimmed phone, got %s", alice.Phone)
	}
	if alice.IsInvalid {
		t.Errorf("alice should not be invalid")
	}

	// Filter by search query (name)
	filterName, err := repo.List(ListFilter{Query: "Alice"})
	if err != nil || len(filterName) != 1 || filterName[0].ID != id1 {
		t.Errorf("filter by name failed, got: %v, err: %v", filterName, err)
	}

	// Filter by category
	filterCat, err := repo.List(ListFilter{Segment: "Tech"})
	if err != nil || len(filterCat) != 2 {
		t.Errorf("filter by category failed, got %d results, err: %v", len(filterCat), err)
	}
}

func TestCampaignTargetsAndRecordSend(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// Lead A: valid uncontacted
	idA, _ := repo.Create(Input{Name: "Target A", Email: "a@example.com", Segment: "VIP"})
	// Lead B: marked invalid
	idB, _ := repo.Create(Input{Name: "Target B", Email: "b@example.com", Segment: "VIP"})
	repo.MarkInvalid(idB)
	// Lead C: valid uncontacted in different category
	idC, _ := repo.Create(Input{Name: "Target C", Email: "c@example.com", Segment: "Regular"})

	// Campaign: mode "initial", category "VIP"
	targets, err := repo.CampaignTargets(Campaign{
		Mode:    "initial",
		Segment: "VIP",
	})
	if err != nil {
		t.Fatalf("CampaignTargets failed: %v", err)
	}
	if len(targets) != 1 || targets[0].ID != idA {
		t.Fatalf("expected 1 target (Lead A), got %v", targets)
	}

	// Record initial send for Lead A
	c := Campaign{Mode: "initial", Subject: "Welcome", Body: "Hello A"}
	if err := repo.RecordSend(SentMessage{LeadID: idA, Mode: c.Mode, Subject: c.Subject, Body: c.Body}); err != nil {
		t.Fatalf("RecordSend initial failed: %v", err)
	}

	// Verify events for Lead A
	events, err := repo.Events(idA)
	if err != nil || len(events) != 1 {
		t.Fatalf("expected 1 event for Lead A, got %v, err: %v", events, err)
	}
	if events[0].Kind != "initial" || events[0].Subject != "Welcome" {
		t.Errorf("unexpected event content: %+v", events[0])
	}

	// Now Lead A should NOT appear in initial targets
	targets, err = repo.CampaignTargets(Campaign{Mode: "initial"})
	if err != nil {
		t.Fatalf("CampaignTargets initial failed: %v", err)
	}
	if len(targets) != 1 || targets[0].ID != idC {
		t.Fatalf("expected only Lead C in initial targets, got %v", targets)
	}

	// Lead A SHOULD appear in followup targets
	targets, err = repo.CampaignTargets(Campaign{Mode: "followup"})
	if err != nil {
		t.Fatalf("CampaignTargets followup failed: %v", err)
	}
	if len(targets) != 1 || targets[0].ID != idA {
		t.Fatalf("expected Lead A in followup targets, got %v", targets)
	}

	// Send followup 1 to Lead A
	fCampaign := Campaign{Mode: "followup", Subject: "Followup 1", Body: "Checking in"}
	if err := repo.RecordSend(SentMessage{LeadID: idA, Mode: fCampaign.Mode, Subject: fCampaign.Subject, Body: fCampaign.Body}); err != nil {
		t.Fatalf("RecordSend followup failed: %v", err)
	}

	// Test NoFollowup filter: Lead A has any_followup=1, so with NoFollowup=true it shouldn't match
	targets, err = repo.CampaignTargets(Campaign{Mode: "followup", NoFollowup: true})
	if err != nil {
		t.Fatalf("CampaignTargets with NoFollowup failed: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets with NoFollowup=true, got %d", len(targets))
	}

	// Test MaxFollowups: Lead A has followup_count = 1.
	// MaxFollowups=1 means followup_count < 1, so it shouldn't match.
	targets, err = repo.CampaignTargets(Campaign{Mode: "followup", MaxFollowups: 1})
	if err != nil {
		t.Fatalf("CampaignTargets MaxFollowups failed: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets when followup_count is 1 and MaxFollowups is 1, got %d", len(targets))
	}

	// MaxFollowups=2 means followup_count < 2, so it should match!
	targets, err = repo.CampaignTargets(Campaign{Mode: "followup", MaxFollowups: 2})
	if err != nil {
		t.Fatalf("CampaignTargets MaxFollowups=2 failed: %v", err)
	}
	if len(targets) != 1 || targets[0].ID != idA {
		t.Errorf("expected Lead A to match MaxFollowups=2, got %v", targets)
	}
}

func TestRecordReplyMarkInvalidMarkOpenedAndStats(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	id, _ := repo.Create(Input{Name: "Prospect", Email: "prospect@example.com"})

	// Initially stats: Total 1, others 0
	stats, err := repo.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.Total != 1 || stats.Emailed != 0 || stats.Opened != 0 || stats.Replied != 0 || stats.Invalid != 0 {
		t.Errorf("unexpected initial stats: %+v", stats)
	}

	// Mark opened
	if err := repo.MarkOpened(id); err != nil {
		t.Fatalf("MarkOpened failed: %v", err)
	}
	stats, _ = repo.Stats()
	if stats.Opened != 1 {
		t.Errorf("expected stats.Opened = 1, got %d", stats.Opened)
	}

	// Record reply
	if err := repo.RecordReply(id, "I am interested!"); err != nil {
		t.Fatalf("RecordReply failed: %v", err)
	}
	stats, _ = repo.Stats()
	if stats.Replied != 1 {
		t.Errorf("expected stats.Replied = 1, got %d", stats.Replied)
	}

	// Mark invalid
	if err := repo.MarkInvalid(id); err != nil {
		t.Fatalf("MarkInvalid failed: %v", err)
	}
	stats, _ = repo.Stats()
	if stats.Invalid != 1 {
		t.Errorf("expected stats.Invalid = 1, got %d", stats.Invalid)
	}

	// Verify events
	events, err := repo.Events(id)
	if err != nil {
		t.Fatalf("Events failed: %v", err)
	}
	if len(events) != 2 { // 'opened' and 'reply'
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	// Events are returned oldest first so they read like a conversation.
	if events[1].Kind != "reply" || events[1].Content != "I am interested!" {
		t.Errorf("unexpected newest event: %+v", events[1])
	}
	if events[0].Kind != "opened" {
		t.Errorf("unexpected older event: %+v", events[0])
	}
}

func TestProcessWebhook(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	id, _ := repo.Create(Input{Name: "Webhook Target", Email: "lead@example.com"})

	// 1. Process duplicate receipt
	receiptID := "rcpt_123"
	event := WebhookEvent{
		Type:      "email.delivered",
		Recipient: "lead@example.com",
		Subject:   "Update",
	}

	processed, err := repo.ProcessWebhook(receiptID, event)
	if err != nil || !processed {
		t.Fatalf("expected processed=true, got %v, err: %v", processed, err)
	}

	// Retry with same receiptID should return false (ignored as duplicate)
	processed, err = repo.ProcessWebhook(receiptID, event)
	if err != nil || processed {
		t.Fatalf("expected processed=false on duplicate receipt, got %v, err: %v", processed, err)
	}

	// 2. Webhook for unknown email
	processed, err = repo.ProcessWebhook("rcpt_unknown", WebhookEvent{
		Type:      "email.delivered",
		Recipient: "unknown@example.com",
	})
	if err != nil || processed {
		t.Fatalf("expected processed=false for unknown email, got %v, err: %v", processed, err)
	}

	// 3. Webhook email.opened
	processed, err = repo.ProcessWebhook("rcpt_open", WebhookEvent{
		Type:      "email.opened",
		Recipient: "lead@example.com",
	})
	if err != nil || !processed {
		t.Fatalf("email.opened webhook failed: %v, err: %v", processed, err)
	}

	leads, _ := repo.List(ListFilter{})
	if !leads[0].IsOpened {
		t.Errorf("expected lead to be marked opened via webhook")
	}

	// 4. Webhook email.received (reply, where Sender is the lead's email)
	processed, err = repo.ProcessWebhook("rcpt_reply", WebhookEvent{
		Type:    "email.received",
		Sender:  "lead@example.com",
		Subject: "Re: Info",
		Content: "Sounds great!",
	})
	if err != nil || !processed {
		t.Fatalf("email.received webhook failed: %v, err: %v", processed, err)
	}

	leads, _ = repo.List(ListFilter{})
	if !leads[0].Replied {
		t.Errorf("expected lead to be marked replied via webhook")
	}

	// 5. Webhook email.bounced
	processed, err = repo.ProcessWebhook("rcpt_bounce", WebhookEvent{
		Type:          "email.bounced",
		Recipient:     "lead@example.com",
		BounceType:    "hard_bounce",
		BounceMessage: "Mailbox does not exist",
	})
	if err != nil || !processed {
		t.Fatalf("email.bounced webhook failed: %v, err: %v", processed, err)
	}

	leads, _ = repo.List(ListFilter{})
	if !leads[0].IsInvalid {
		t.Errorf("expected lead to be marked invalid on bounce")
	}

	// Check events recorded
	events, err := repo.Events(id)
	if err != nil {
		t.Fatalf("Events failed: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("expected 4 webhook events, got %d", len(events))
	}
}
