package lead

type Lead struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Category       string `json:"category"`
	Subcategory    string `json:"subcategory"`
	MailSent       bool   `json:"mailSent"`
	IsInvalid      bool   `json:"isInvalid"`
	IsOpened       bool   `json:"isOpened"`
	AnyFollowup    bool   `json:"anyFollowup"`
	FollowupCount  int    `json:"followupCount"`
	FollowupLimit  int    `json:"followupLimit"`
	LastFollowupAt string `json:"lastFollowupAt"`
	NextFollowupAt string `json:"nextFollowupAt"`
	FollowupPaused bool   `json:"followupPaused"`
	Replied        bool   `json:"replied"`
	CreatedAt      string `json:"createdAt"`
}

type Input struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
}

type ListFilter struct {
	Query    string
	Category string
}

type Campaign struct {
	Subject           string `json:"subject"`
	Body              string `json:"body"`
	Mode              string `json:"mode"`
	Before            string `json:"before"`
	Category          string `json:"category"`
	NoFollowup        bool   `json:"noFollowup"`
	FollowupDelayDays int    `json:"followupDelayDays"`
	MaxFollowups      int    `json:"maxFollowups"`
}

type Event struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

type Stats struct {
	Total   int `json:"total"`
	Emailed int `json:"emailed"`
	Opened  int `json:"opened"`
	Replied int `json:"replied"`
	Invalid int `json:"invalid"`
}

// WebhookEvent is the normalized subset of a Resend event used by the application.
type WebhookEvent struct {
	Type          string
	Sender        string
	Recipient     string
	Subject       string
	MessageID     string
	Content       string
	BounceType    string
	BounceMessage string
}
