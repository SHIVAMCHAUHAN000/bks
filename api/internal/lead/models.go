package lead

type Lead struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Organization   string `json:"organization"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Designation    string `json:"designation"`
	Address        string `json:"address"`
	Segment        string `json:"segment"`
	MailSent       bool   `json:"mailSent"`
	IsInvalid      bool   `json:"isInvalid"`
	IsOpened       bool   `json:"isOpened"`
	AnyFollowup    bool   `json:"anyFollowup"`
	FollowupCount  int    `json:"followupCount"`
	Replied        bool   `json:"replied"`
	Unsubscribed   bool   `json:"unsubscribed"`
	UnsubToken     string `json:"-"`
	LastActivityAt string `json:"lastActivityAt"`
	CreatedAt      string `json:"createdAt"`
}

// Input holds the lead fields that can be entered manually or imported.
type Input struct {
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Designation  string `json:"designation"`
	Address      string `json:"address"`
	Segment      string `json:"segment"`
}

type ListFilter struct {
	Query   string
	Segment string
	IDs     []int64
}

type Campaign struct {
	Subject            string `json:"subject"`
	Body               string `json:"body"`
	Mode               string `json:"mode"`
	Before             string `json:"before"`
	Segment            string `json:"segment"`
	NoFollowup         bool   `json:"noFollowup"`
	MaxFollowups       int    `json:"maxFollowups"`
	TrackOpens         bool   `json:"trackOpens"`
	IncludeUnsubscribe bool   `json:"includeUnsubscribe"`
}

// CampaignRun is a campaign that is sent in the background.
type CampaignRun struct {
	ID         int64  `json:"id"`
	Mode       string `json:"mode"`
	Subject    string `json:"subject"`
	Segment    string `json:"segment"`
	Status     string `json:"status"`
	Total      int    `json:"total"`
	Sent       int    `json:"sent"`
	Failed     int    `json:"failed"`
	LastError  string `json:"lastError"`
	CreatedAt  string `json:"createdAt"`
	FinishedAt string `json:"finishedAt"`
}

type Event struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Content   string `json:"content"`
	From      string `json:"from"`
	To        string `json:"to"`
	ResendID  string `json:"resendId"`
	CreatedAt string `json:"createdAt"`
}

// Conversation is one row in the inbox list.
type Conversation struct {
	Lead        Lead   `json:"lead"`
	LastKind    string `json:"lastKind"`
	LastSubject string `json:"lastSubject"`
	LastSnippet string `json:"lastSnippet"`
	LastAt      string `json:"lastAt"`
	Messages    int    `json:"messages"`
	Replies     int    `json:"replies"`
}

type Stats struct {
	Total        int `json:"total"`
	Emailed      int `json:"emailed"`
	Opened       int `json:"opened"`
	Replied      int `json:"replied"`
	Invalid      int `json:"invalid"`
	Unsubscribed int `json:"unsubscribed"`
	SentToday    int `json:"sentToday"`
}

// ImportResult describes what happened to one imported row.
type ImportResult struct {
	Row    int    `json:"row"`
	Status string `json:"status"` // added, duplicate, invalid, empty
	Lead   Input  `json:"lead"`
	ID     int64  `json:"id,omitempty"`
}

// WebhookEvent is the normalized subset of a Resend event used by the application.
type WebhookEvent struct {
	Type          string
	EmailID       string
	Sender        string
	Recipient     string
	Subject       string
	MessageID     string
	Content       string
	BounceType    string
	BounceMessage string
}
