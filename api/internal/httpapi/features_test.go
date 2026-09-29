package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"automationtool/api/internal/lead"
)

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body == nil {
		reader = &bytes.Buffer{}
	} else if raw, ok := body.(string); ok {
		reader = bytes.NewBufferString(raw)
	} else {
		data, _ := json.Marshal(body)
		reader = bytes.NewBuffer(data)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, reader))
	return rec
}

func TestDeleteEndpoints(t *testing.T) {
	server, repo, _, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	h := server.Routes()
	a, _ := repo.Create(lead.Input{Email: "a@example.com"})
	b, _ := repo.Create(lead.Input{Email: "b@example.com"})
	c, _ := repo.Create(lead.Input{Email: "c@example.com"})
	if rec := do(t, h, http.MethodDelete, fmt.Sprintf("/api/leads/%d", a), nil); rec.Code != 200 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodDelete, fmt.Sprintf("/api/leads/%d", a), nil); rec.Code != 404 {
		t.Errorf("expected 404 for already deleted lead, got %d", rec.Code)
	}
	rec := do(t, h, http.MethodPost, "/api/leads/delete", map[string]any{"ids": []int64{b, c}})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"deleted":2`) {
		t.Fatalf("bulk delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPost, "/api/leads/delete", map[string]any{"ids": []int64{}}); rec.Code != 400 {
		t.Errorf("expected 400 for empty ids, got %d", rec.Code)
	}
}

func TestImportMapsMessyColumnsToLeadFields(t *testing.T) {
	server, repo, _, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	h := server.Routes()
	csv := "\xef\xbb\xbfS.No,Company Name,Contact Person,Email ID,Mobile No.,Job Title,City,Turnover\n" +
		"1,Acme Pvt Ltd,Mr. Ravi Kumar,Ravi@Acme.in,9876543210.0,Director,Pune,10Cr\n" +
		"2,Beta Corp,Sara Khan,sara@beta.com; info@beta.com,,CEO,Delhi\n" + // short row
		"\n" +
		"3,Gamma,No Email,,,Manager,Goa,1Cr\n"
	rec := do(t, h, http.MethodPost, "/api/import/preview", map[string]any{"csv": csv, "segment": "Manufacturing"})
	if rec.Code != 200 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var preview struct {
		Mapping   map[string]string
		Ignored   []string
		Rows      int
		WithEmail int
		Sample    []lead.Input
	}
	json.Unmarshal(rec.Body.Bytes(), &preview)
	if preview.Mapping["organization"] != "Company Name" || preview.Mapping["name"] != "Contact Person" || preview.Mapping["email"] != "Email ID" ||
		preview.Mapping["phone"] != "Mobile No." || preview.Mapping["designation"] != "Job Title" || preview.Mapping["address"] != "City" {
		t.Errorf("unexpected mapping: %v", preview.Mapping)
	}
	if preview.Rows != 3 || preview.WithEmail != 2 || len(preview.Ignored) != 2 {
		t.Errorf("unexpected preview: %+v", preview)
	}
	if preview.Sample[1].Email != "sara@beta.com" || preview.Sample[0].Phone != "9876543210" || preview.Sample[0].Segment != "Manufacturing" {
		t.Errorf("unexpected sample: %+v", preview.Sample)
	}

	// Manually skip the address column when importing.
	rec = do(t, h, http.MethodPost, "/api/import", map[string]any{"csv": csv, "segment": "Manufacturing", "mapping": map[string]string{"address": ""}})
	if rec.Code != 200 {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	leads, _ := repo.List(lead.ListFilter{})
	// Gamma has neither an email nor a phone number, so it is skipped.
	if len(leads) != 2 {
		t.Fatalf("expected 2 leads, got %d", len(leads))
	}
	ravi := leads[1]
	if ravi.Name != "Mr. Ravi Kumar" || ravi.Organization != "Acme Pvt Ltd" || ravi.Email != "ravi@acme.in" || ravi.Designation != "Director" || ravi.Address != "" || ravi.Segment != "Manufacturing" {
		t.Errorf("unexpected lead: %+v", ravi)
	}
}

func TestImportRejectsRowsWithoutContact(t *testing.T) {
	server, repo, _, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	h := server.Routes()
	rec := do(t, h, http.MethodPost, "/api/import", map[string]any{"csv": "Company,Email\nA,a@a.com\nB,\n"})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"empty":1`) {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	leads, _ := repo.List(lead.ListFilter{})
	if len(leads) != 1 || leads[0].Organization != "A" {
		t.Errorf("unexpected leads: %+v", leads)
	}
}

func TestPersonalisedCampaignWithUnsubscribe(t *testing.T) {
	server, repo, mail, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	h := server.Routes()
	id, _ := repo.Create(lead.Input{Name: "Dr. Meera Iyer", Organization: "City Hospital", Designation: "Director", Address: "Chennai", Email: "meera@city.org", Segment: "Hospitals"})
	repo.Create(lead.Input{Name: "Other", Email: "other@school.org", Segment: "Schools"})

	campaign := map[string]any{"mode": "initial", "segment": "Hospitals", "subject": "{{organization}} x Omnidel", "body": "Hi {{first_name|there}},\n\nAs {{designation}} at {{organization}} in {{address}}...", "includeUnsubscribe": true, "trackOpens": false}
	rec := do(t, h, http.MethodPost, "/api/automation/preview", campaign)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"eligible":1`) || !strings.Contains(rec.Body.String(), "City Hospital x Omnidel") {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}

	bad := map[string]any{"subject": "Hi {{nmae}}", "body": "x"}
	if rec := do(t, h, http.MethodPost, "/api/automation/send", bad); rec.Code != 400 || !strings.Contains(rec.Body.String(), "nmae") {
		t.Errorf("expected unknown placeholder error, got %d %s", rec.Code, rec.Body.String())
	}

	if rec := do(t, h, http.MethodPost, "/api/automation/send", campaign); rec.Code != http.StatusAccepted {
		t.Fatalf("send: %d %s", rec.Code, rec.Body.String())
	}
	server.Wait()
	if len(mail.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(mail.sent))
	}
	m := mail.sent[0]
	if m.To != "meera@city.org" || m.Subject != "City Hospital x Omnidel" || !strings.HasPrefix(m.Text, "Hi Meera,\n\nAs Director at City Hospital in Chennai...") {
		t.Errorf("unexpected message: %+v", m)
	}
	if !strings.Contains(m.Headers["List-Unsubscribe"], "https://api.example.com/unsubscribe/") || strings.Contains(m.HTML, "/track/open/") {
		t.Errorf("unexpected headers/html: %v %s", m.Headers, m.HTML)
	}

	// The exact personalised email is stored in the conversation.
	events, _ := repo.Events(id)
	if len(events) != 1 || events[0].Subject != "City Hospital x Omnidel" || !strings.Contains(events[0].Body, "Hi Meera") || events[0].ResendID != "em_1" {
		t.Errorf("unexpected stored event: %+v", events)
	}
	rec = do(t, h, http.MethodGet, "/api/conversations", nil)
	if !strings.Contains(rec.Body.String(), "meera@city.org") || strings.Contains(rec.Body.String(), "other@school.org") {
		t.Errorf("unexpected conversations: %s", rec.Body.String())
	}

	// Following the unsubscribe link excludes the lead from follow-ups.
	link := m.Headers["List-Unsubscribe"]
	path := strings.TrimSuffix(link[strings.Index(link, "/unsubscribe/"):], ">")
	if rec := do(t, h, http.MethodGet, path, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "You have been unsubscribed") {
		t.Errorf("unsubscribe page: %d %s", rec.Code, rec.Body.String())
	}
	if targets, _ := repo.CampaignTargets(lead.Campaign{Mode: "followup"}); len(targets) != 0 {
		t.Errorf("unsubscribed lead should not get follow-ups")
	}
	if rec := do(t, h, http.MethodGet, "/api/settings", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"sentToday":1`) {
		t.Errorf("settings: %s", rec.Body.String())
	}
}

func TestDailyLimitStopsCampaign(t *testing.T) {
	server, repo, mail, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	server.Configure(SendSettings{PublicURL: "https://api.example.com", DailyLimit: 2})
	h := server.Routes()
	for i := 0; i < 4; i++ {
		repo.Create(lead.Input{Email: fmt.Sprintf("l%d@example.com", i)})
	}
	rec := do(t, h, http.MethodPost, "/api/automation/send", map[string]any{"subject": "Hi", "body": "Hello {{name|there}}"})
	var run lead.CampaignRun
	json.Unmarshal(rec.Body.Bytes(), &run)
	server.Wait()
	run, _ = repo.CampaignRun(run.ID)
	if run.Status != "daily_limit" || run.Sent != 2 || len(mail.sent) != 2 {
		t.Errorf("unexpected run: %+v", run)
	}
}
