package models

// MailMessage is one inbound email stored by the mail intake API
// (POST /mail/inbound). The JSON field names mirror the local dev mailsink
// contract so orchid's mail MCP reads this backend unchanged. ts is an
// ISO-8601 UTC string; id is a 12-char hex id generated on insert.
type MailMessage struct {
	ID        string `json:"id"`
	TS        string `json:"ts"`
	FromAddr  string `json:"from_addr"`
	ToAddr    string `json:"to_addr"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	MessageID string `json:"-"`
}

// MailSummary is the list-view shape of a stored mail (everything except body).
type MailSummary struct {
	ID       string `json:"id"`
	TS       string `json:"ts"`
	FromAddr string `json:"from_addr"`
	ToAddr   string `json:"to_addr"`
	Subject  string `json:"subject"`
}
