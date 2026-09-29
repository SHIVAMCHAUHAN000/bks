package lead

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
)

const leadColumns = `id,name,organization,email,phone,designation,address,segment,mail_sent,is_invalid,is_opened,any_followup,followup_count,replied,unsubscribed,unsub_token,created_at,
	COALESCE((SELECT MAX(created_at) FROM email_events e WHERE e.lead_id=leads.id), '')`
const selectLeads = `SELECT ` + leadColumns + ` FROM leads`

var (
	ErrInvalidEmail = errors.New("invalid email address")
	ErrMissingEmail = errors.New("email or phone is required")
	ErrDuplicate    = errors.New("a lead with this email or phone already exists")
	ErrNotFound     = errors.New("lead not found")
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// Normalize trims values and validates the email address.
func Normalize(input Input) (Input, error) {
	input.Name = strings.Join(strings.Fields(input.Name), " ")
	input.Organization = strings.TrimSpace(input.Organization)
	input.Designation = strings.TrimSpace(input.Designation)
	input.Address = strings.TrimSpace(input.Address)
	input.Segment = strings.TrimSpace(input.Segment)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	if input.Email == "" && input.Phone == "" {
		return input, ErrMissingEmail
	}
	if input.Email != "" {
		parsed, err := mail.ParseAddress(input.Email)
		if err != nil || parsed.Address != input.Email || len(input.Email) > 254 || !strings.Contains(input.Email[strings.LastIndex(input.Email, "@"):], ".") {
			return input, ErrInvalidEmail
		}
	}
	return input, nil
}

func insertLead(db execer, input Input) (int64, error) {
	input, err := Normalize(input)
	if err != nil {
		return 0, err
	}
	result, err := db.Exec(`INSERT INTO leads(name,organization,email,phone,designation,address,segment,is_invalid) VALUES(?,?,?,?,?,?,?,0)`,
		input.Name, input.Organization, input.Email, input.Phone, input.Designation, input.Address, input.Segment)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return 0, ErrDuplicate
		}
		return 0, err
	}
	return result.LastInsertId()
}

func (r *Repository) Create(input Input) (int64, error) { return insertLead(r.db, input) }

// Import inserts many leads in one transaction and reports the outcome of every row.
func (r *Repository) Import(inputs []Input, firstRow int) ([]ImportResult, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	results := make([]ImportResult, 0, len(inputs))
	for i, input := range inputs {
		result := ImportResult{Row: firstRow + i, Lead: input}
		id, err := insertLead(tx, input)
		switch {
		case err == nil:
			result.Status, result.ID = "added", id
		case errors.Is(err, ErrMissingEmail):
			result.Status = "empty"
		case errors.Is(err, ErrInvalidEmail):
			result.Status = "invalid"
		case errors.Is(err, ErrDuplicate):
			result.Status = "duplicate"
		default:
			return nil, err
		}
		if normalized, nErr := Normalize(input); nErr == nil {
			result.Lead = normalized
		}
		results = append(results, result)
	}
	return results, tx.Commit()
}

func (r *Repository) Get(id int64) (Lead, error) {
	leads, err := scanLeads(r.db.Query(selectLeads+` WHERE id=?`, id))
	if err != nil {
		return Lead{}, err
	}
	if len(leads) == 0 {
		return Lead{}, ErrNotFound
	}
	return leads[0], nil
}

func (r *Repository) List(filter ListFilter) ([]Lead, error) {
	query, segment := "%"+filter.Query+"%", "%"+filter.Segment+"%"
	return scanLeads(r.db.Query(selectLeads+` WHERE (email LIKE ? OR phone LIKE ? OR name LIKE ? OR organization LIKE ? OR designation LIKE ?) AND segment LIKE ? ORDER BY id DESC`,
		query, query, query, query, query, segment))
}

// Delete removes leads and their email history.
func (r *Repository) Delete(ids ...int64) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	deleted := 0
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM email_events WHERE lead_id=?`, id); err != nil {
			return 0, err
		}
		result, err := tx.Exec(`DELETE FROM leads WHERE id=?`, id)
		if err != nil {
			return 0, err
		}
		n, _ := result.RowsAffected()
		deleted += int(n)
	}
	return deleted, tx.Commit()
}

func (r *Repository) CampaignTargets(c Campaign) ([]Lead, error) {
	where := ` WHERE is_invalid=0 AND replied=0 AND unsubscribed=0 AND email != ''`
	args := []any{}
	if c.Mode == "followup" {
		where += ` AND mail_sent=1`
	} else {
		where += ` AND mail_sent=0`
	}
	if c.NoFollowup {
		where += ` AND any_followup=0`
	}
	if c.Mode == "followup" && c.MaxFollowups > 0 {
		where += ` AND followup_count < ?`
		args = append(args, c.MaxFollowups)
	}
	if c.Segment != "" {
		where += ` AND segment=?`
		args = append(args, c.Segment)
	}
	if c.Before != "" {
		where += ` AND date(created_at)<=date(?)`
		args = append(args, c.Before)
	}
	return scanLeads(r.db.Query(selectLeads+where+` ORDER BY id`, args...))
}

// EnsureUnsubToken returns the lead's unsubscribe token, creating one if needed.
func (r *Repository) EnsureUnsubToken(id int64) (string, error) {
	var token string
	if err := r.db.QueryRow(`SELECT unsub_token FROM leads WHERE id=?`, id).Scan(&token); err != nil {
		return "", err
	}
	if token != "" {
		return token, nil
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token = hex.EncodeToString(buf)
	_, err := r.db.Exec(`UPDATE leads SET unsub_token=? WHERE id=?`, token, id)
	return token, err
}

// Unsubscribe marks the lead with this token as unsubscribed.
func (r *Repository) Unsubscribe(token string) (bool, error) {
	if len(token) < 16 {
		return false, nil
	}
	var id int64
	var already int
	err := r.db.QueryRow(`SELECT id, unsubscribed FROM leads WHERE unsub_token=?`, token).Scan(&id, &already)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil || already == 1 {
		return err == nil, err
	}
	if _, err := r.db.Exec(`UPDATE leads SET unsubscribed=1 WHERE id=?`, id); err != nil {
		return false, err
	}
	_, err = r.db.Exec(`INSERT INTO email_events(lead_id,kind,content) VALUES(?, 'unsubscribed', 'The lead used the unsubscribe link.')`, id)
	return true, err
}

// SentMessage is one personalised email that was accepted by the mail provider.
type SentMessage struct {
	LeadID     int64
	CampaignID int64
	Mode       string
	Subject    string
	Body       string
	From       string
	To         string
	ResendID   string
}

func (r *Repository) RecordSend(m SentMessage) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if m.Mode == "followup" {
		_, err = tx.Exec(`UPDATE leads SET any_followup=1, followup_count=followup_count+1, last_followup_at=CURRENT_TIMESTAMP WHERE id=?`, m.LeadID)
	} else {
		m.Mode = "initial"
		_, err = tx.Exec(`UPDATE leads SET mail_sent=1 WHERE id=?`, m.LeadID)
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO email_events(lead_id,kind,subject,body,from_addr,to_addr,resend_id,campaign_id) VALUES(?,?,?,?,?,?,?,?)`,
		m.LeadID, m.Mode, m.Subject, m.Body, m.From, m.To, m.ResendID, m.CampaignID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) RecordFailure(leadID, campaignID int64, subject, reason string) error {
	_, err := r.db.Exec(`INSERT INTO email_events(lead_id,kind,subject,content,campaign_id) VALUES(?,'failed',?,?,?)`, leadID, subject, reason, campaignID)
	return err
}

// Events returns a lead's history, oldest first, so it reads like a conversation.
func (r *Repository) Events(id int64) ([]Event, error) {
	rows, err := r.db.Query(`SELECT id,kind,subject,body,content,from_addr,to_addr,resend_id,created_at FROM email_events WHERE lead_id=? ORDER BY created_at, id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Kind, &e.Subject, &e.Body, &e.Content, &e.From, &e.To, &e.ResendID, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Conversations lists leads that have been emailed or have replied, newest activity first.
func (r *Repository) Conversations(onlyReplied bool, query string) ([]Conversation, error) {
	where := `WHERE (l.mail_sent=1 OR l.replied=1 OR l.any_followup=1)`
	if onlyReplied {
		where = `WHERE l.replied=1`
	}
	like := "%" + query + "%"
	rows, err := r.db.Query(`SELECT l.id,
			COALESCE(last.kind,''), COALESCE(last.subject,''), COALESCE(CASE WHEN last.content!='' THEN last.content ELSE last.body END,''), COALESCE(last.created_at,''),
			(SELECT COUNT(*) FROM email_events e WHERE e.lead_id=l.id AND e.kind IN ('initial','followup','reply')),
			(SELECT COUNT(*) FROM email_events e WHERE e.lead_id=l.id AND e.kind='reply')
		FROM leads l
		LEFT JOIN email_events last ON last.id = (SELECT e.id FROM email_events e WHERE e.lead_id=l.id AND e.kind IN ('initial','followup','reply') ORDER BY e.created_at DESC, e.id DESC LIMIT 1)
		`+where+` AND (l.email LIKE ? OR l.name LIKE ? OR l.organization LIKE ?)
		ORDER BY COALESCE(last.created_at,'') DESC, l.id DESC`, like, like, like)
	if err != nil {
		return nil, err
	}
	type row struct {
		id int64
		c  Conversation
	}
	collected := []row{}
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.c.LastKind, &item.c.LastSubject, &item.c.LastSnippet, &item.c.LastAt, &item.c.Messages, &item.c.Replies); err != nil {
			rows.Close()
			return nil, err
		}
		if len([]rune(item.c.LastSnippet)) > 160 {
			item.c.LastSnippet = string([]rune(item.c.LastSnippet)[:160]) + "…"
		}
		collected = append(collected, item)
	}
	rows.Close()
	leads, err := scanLeads(r.db.Query(selectLeads + ` WHERE mail_sent=1 OR replied=1 OR any_followup=1`))
	if err != nil {
		return nil, err
	}
	byID := map[int64]Lead{}
	for _, l := range leads {
		byID[l.ID] = l
	}
	conversations := make([]Conversation, 0, len(collected))
	for _, item := range collected {
		item.c.Lead = byID[item.id]
		conversations = append(conversations, item.c)
	}
	return conversations, nil
}

func (r *Repository) RecordReply(id int64, content string) error {
	if _, err := r.db.Exec(`UPDATE leads SET replied=1 WHERE id=?`, id); err != nil {
		return err
	}
	_, err := r.db.Exec(`INSERT INTO email_events(lead_id,kind,content) VALUES(?, 'reply', ?)`, id, content)
	return err
}
func (r *Repository) MarkInvalid(id int64) error {
	_, err := r.db.Exec(`UPDATE leads SET is_invalid=1 WHERE id=?`, id)
	return err
}
func (r *Repository) MarkOpened(id int64) error {
	var exists int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM leads WHERE id=?`, id).Scan(&exists); err != nil || exists == 0 {
		return err
	}
	if _, err := r.db.Exec(`UPDATE leads SET is_opened=1 WHERE id=?`, id); err != nil {
		return err
	}
	_, err := r.db.Exec(`INSERT INTO email_events(lead_id,kind) VALUES(?, 'opened')`, id)
	return err
}

// SentToday counts emails accepted by the provider since midnight UTC.
func (r *Repository) SentToday() (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM email_events WHERE kind IN ('initial','followup') AND date(created_at)=date('now')`).Scan(&count)
	return count, err
}

func (r *Repository) Stats() (Stats, error) {
	var stats Stats
	err := r.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(mail_sent),0),COALESCE(SUM(is_opened),0),COALESCE(SUM(replied),0),COALESCE(SUM(is_invalid),0),COALESCE(SUM(unsubscribed),0) FROM leads`).
		Scan(&stats.Total, &stats.Emailed, &stats.Opened, &stats.Replied, &stats.Invalid, &stats.Unsubscribed)
	if err != nil {
		return stats, err
	}
	stats.SentToday, err = r.SentToday()
	return stats, err
}

// Campaign runs.

func (r *Repository) CreateCampaignRun(c Campaign, total int) (int64, error) {
	result, err := r.db.Exec(`INSERT INTO campaigns(mode,subject,body,segment,status,total) VALUES(?,?,?,?, 'running', ?)`, c.Mode, c.Subject, c.Body, c.Segment, total)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *Repository) UpdateCampaignRun(id int64, sent, failed int, lastError string) error {
	_, err := r.db.Exec(`UPDATE campaigns SET sent=?, failed=?, last_error=? WHERE id=?`, sent, failed, lastError, id)
	return err
}

func (r *Repository) FinishCampaignRun(id int64, status string) error {
	_, err := r.db.Exec(`UPDATE campaigns SET status=?, finished_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

const selectCampaigns = `SELECT id,mode,subject,segment,status,total,sent,failed,last_error,created_at,finished_at FROM campaigns`

func (r *Repository) CampaignRun(id int64) (CampaignRun, error) {
	runs, err := r.scanCampaigns(r.db.Query(selectCampaigns+` WHERE id=?`, id))
	if err != nil {
		return CampaignRun{}, err
	}
	if len(runs) == 0 {
		return CampaignRun{}, sql.ErrNoRows
	}
	return runs[0], nil
}

func (r *Repository) CampaignRuns(limit int) ([]CampaignRun, error) {
	return r.scanCampaigns(r.db.Query(selectCampaigns+` ORDER BY id DESC LIMIT ?`, limit))
}

func (r *Repository) scanCampaigns(rows *sql.Rows, err error) ([]CampaignRun, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []CampaignRun{}
	for rows.Next() {
		var c CampaignRun
		if err := rows.Scan(&c.ID, &c.Mode, &c.Subject, &c.Segment, &c.Status, &c.Total, &c.Sent, &c.Failed, &c.LastError, &c.CreatedAt, &c.FinishedAt); err != nil {
			return nil, err
		}
		runs = append(runs, c)
	}
	return runs, rows.Err()
}

// ProcessWebhook writes an activity record once, even when Resend retries the same Svix message.
func (r *Repository) ProcessWebhook(receiptID string, event WebhookEvent) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT OR IGNORE INTO webhook_receipts(id,event_type) VALUES(?,?)`, receiptID, event.Type)
	if err != nil {
		return false, err
	}
	inserted, _ := result.RowsAffected()
	if inserted == 0 {
		return false, tx.Commit()
	}

	var leadID int64
	err = sql.ErrNoRows
	// Delivery events carry the id of the email we sent; match on that first.
	if event.Type != "email.received" && event.EmailID != "" {
		err = tx.QueryRow(`SELECT lead_id FROM email_events WHERE resend_id=? LIMIT 1`, event.EmailID).Scan(&leadID)
	}
	if err == sql.ErrNoRows {
		address := event.Recipient
		if event.Type == "email.received" {
			address = event.Sender
		}
		err = tx.QueryRow(`SELECT id FROM leads WHERE email = ?`, strings.ToLower(address)).Scan(&leadID)
	}
	if err == sql.ErrNoRows {
		return false, tx.Commit()
	}
	if err != nil {
		return false, err
	}

	kind, content := strings.TrimPrefix(event.Type, "email."), event.Content
	from, to := "", ""
	switch event.Type {
	case "email.received":
		kind, from, to = "reply", event.Sender, event.Recipient
		if _, err = tx.Exec(`UPDATE leads SET replied=1 WHERE id=?`, leadID); err != nil {
			return false, err
		}
	case "email.bounced":
		content = strings.Trim(strings.TrimSpace(strings.Join([]string{event.BounceType, event.BounceMessage}, ": ")), ": ")
		if _, err = tx.Exec(`UPDATE leads SET is_invalid=1 WHERE id=?`, leadID); err != nil {
			return false, err
		}
	case "email.opened":
		if _, err = tx.Exec(`UPDATE leads SET is_opened=1 WHERE id=?`, leadID); err != nil {
			return false, err
		}
	}
	if content == "" {
		content = "Resend reported " + kind + "."
	}
	resendID := ""
	if event.Type != "email.received" {
		resendID = event.EmailID
	}
	_, err = tx.Exec(`INSERT INTO email_events(lead_id,kind,subject,content,from_addr,to_addr,resend_id) VALUES(?,?,?,?,?,?,?)`, leadID, kind, event.Subject, content, from, to, resendID)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func scanLeads(rows *sql.Rows, err error) ([]Lead, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	leads := []Lead{}
	for rows.Next() {
		var l Lead
		var mailSent, invalid, opened, followup, replied, unsubscribed int
		if err := rows.Scan(&l.ID, &l.Name, &l.Organization, &l.Email, &l.Phone, &l.Designation, &l.Address, &l.Segment,
			&mailSent, &invalid, &opened, &followup, &l.FollowupCount, &replied, &unsubscribed, &l.UnsubToken, &l.CreatedAt, &l.LastActivityAt); err != nil {
			return nil, err
		}
		l.MailSent = mailSent == 1
		l.IsInvalid = invalid == 1
		l.IsOpened = opened == 1
		l.AnyFollowup = followup == 1
		l.Replied = replied == 1
		l.Unsubscribed = unsubscribed == 1
		leads = append(leads, l)
	}
	return leads, rows.Err()
}
