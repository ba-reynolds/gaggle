package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/ianaindex"
)

const (
	// maxInboundMailBytes caps the raw-MIME body before parsing (spec: ~1 MB).
	maxInboundMailBytes = 1 << 20
)

// MailHandler implements the mail intake contract consumed by orchid's mail
// MCP (and mirrored from the local dev mailsink): the Cloudflare Email Worker
// POSTs raw MIME to /mail/inbound, agents read it back via GET /mails*.
type MailHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewMailHandler(service *service.Service, logger *slog.Logger) *MailHandler {
	return &MailHandler{service: service, logger: logger}
}

// Inbound accepts a raw MIME message. IMPORTANT: any non-2xx makes Cloudflare
// treat delivery as failed and BOUNCE the original sender's mail, so this
// endpoint answers 200 even when the body is oversized or unparseable — it
// stores what it can and logs the rest.
func (h *MailHandler) Inbound(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxInboundMailBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Warn("mail inbound: body unreadable (oversized?)", "error", err)
		writeMailJSON(w, http.StatusOK, map[string]any{"received": false, "reason": "body_unreadable"})
		return
	}

	m, parseErr := parseInboundMail(raw, r.Header.Get("x-orig-to"))
	if parseErr != nil {
		// Still 200 — a bounce is worse than a dropped row.
		h.logger.Warn("mail inbound: parse failed, dropping", "error", parseErr)
		writeMailJSON(w, http.StatusOK, map[string]any{"received": false, "reason": "parse_failed"})
		return
	}
	m.ID = newMailID()
	m.TS = time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")

	inserted, err := h.service.Mail.Insert(r.Context(), m)
	if err != nil {
		h.logger.Error("mail inbound: insert failed", "error", err)
		writeMailJSON(w, http.StatusOK, map[string]any{"received": false, "reason": "store_failed"})
		return
	}
	writeMailJSON(w, http.StatusOK, map[string]any{"received": true, "duplicate": !inserted, "id": m.ID})
}

// ListMails handles GET /mails?to=<substring>&limit=<n>. Response shape is the
// plain {"mails": [...]} contract (no envelope) — the MCP depends on it.
func (h *MailHandler) ListMails(w http.ResponseWriter, r *http.Request) {
	limit := parseMailLimit(r.URL.Query().Get("limit"))
	mails, err := h.service.Mail.List(r.Context(), r.URL.Query().Get("to"), limit)
	if err != nil {
		h.respondError(w, err)
		return
	}
	if mails == nil {
		mails = []models.MailSummary{}
	}
	writeMailJSON(w, http.StatusOK, map[string]any{"mails": mails})
}

// GetMail handles GET /mails/{mailID} — the full record including body.
func (h *MailHandler) GetMail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "mailID")
	m, err := h.service.Mail.GetByID(r.Context(), id)
	if err != nil {
		h.respondError(w, err)
		return
	}
	writeMailJSON(w, http.StatusOK, m)
}

func (h *MailHandler) respondError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr != nil {
		status = appErr.Status
		message = appErr.Message
	}
	writeMailJSON(w, status, map[string]string{"error": message})
}

// parseInboundMail extracts the stored fields from a raw MIME message.
// toOverride is the envelope RCPT TO from the x-orig-to header set by our
// Worker — preferred over the MIME To: header because mailing-list/bcc style
// mail can hide the real recipient (agents filter on exact account@gaggle.land).
func parseInboundMail(raw []byte, toOverride string) (*models.MailMessage, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}

	m := &models.MailMessage{
		MessageID: strings.TrimSpace(msg.Header.Get("Message-ID")),
		Subject:   decodeMimeWords(msg.Header.Get("Subject")),
		FromAddr:  firstAddress(decodeMimeWords(msg.Header.Get("From"))),
		ToAddr:    strings.TrimSpace(toOverride),
	}
	if m.ToAddr == "" {
		m.ToAddr = firstAddress(decodeMimeWords(msg.Header.Get("To")))
	}

	m.Body, m.HTML = extractParts(msg.Header, msg.Body)
	return m, nil
}

// extractParts returns the stored body (stripped text) and html (raw, decoded
// text/html markup) from a MIME message. body is the first text/plain part,
// falling back to stripped text of the first text/html part when a message is
// HTML-only; html is the first text/html part verbatim (hrefs intact) so
// link-based verification flows survive. It NEVER fails hard: structural
// trouble is logged and the message is stored with whatever was recovered
// (spec: store what you have rather than dropping mail — a bounce is worse
// than a partial row).
func extractParts(header mail.Header, body io.Reader) (bodyText, htmlText string) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(mediaType, "multipart/") {
		plain, htmlPart, werr := walkPartsForText(multipart.NewReader(body, params["boundary"]))
		if werr != nil {
			slog.Warn("mail inbound: multipart walk failed", "error", werr)
		}
		return strings.TrimRight(preferText(plain, htmlPart), "\r\n"),
			strings.TrimRight(htmlPart, "\r\n")
	}
	if err == nil && mediaType != "" && mediaType != "text/plain" && mediaType != "text/html" {
		return "", ""
	}
	data, terr := readTransferDecoded(header.Get("Content-Transfer-Encoding"), body)
	if terr != nil {
		slog.Warn("mail inbound: transfer decoding failed", "error", terr)
		return "", ""
	}
	// Multipart parts are transfer-decoded by the reader above; net/mail only
	// unwraps quoted-printable at the top level, so base64 is handled here.
	text, cerr := decodeCharset(params["charset"], data)
	if cerr != nil {
		slog.Warn("mail inbound: charset conversion failed, storing raw bytes",
			"error", cerr, "charset", params["charset"])
	}
	if mediaType == "text/html" {
		return strings.TrimRight(htmlToText(text), "\r\n"), strings.TrimRight(text, "\r\n")
	}
	// Trim the message's final line ending so bodies are uniform regardless of
	// encoding path (verification-code extraction reads this field directly).
	return strings.TrimRight(text, "\r\n"), ""
}

// preferText picks the extracted text/plain body when present, else falls back
// to the stripped text of the first text/html part.
func preferText(plain, htmlPart string) string {
	if strings.TrimSpace(plain) != "" {
		return plain
	}
	if htmlPart != "" {
		return htmlToText(htmlPart)
	}
	return ""
}

// walkPartsForText walks a MIME part tree collecting the first inline
// text/plain part and the first inline text/html part (each independently —
// a verification mail carries both; the HTML holds the links). It recurses
// into nested multipart containers (e.g. multipart/alternative).
func walkPartsForText(mr *multipart.Reader) (plain, htmlPart string, err error) {
	for {
		part, perr := mr.NextPart()
		if perr == io.EOF {
			return plain, htmlPart, nil
		}
		if perr != nil {
			return plain, htmlPart, perr
		}
		ct, params, cterr := mime.ParseMediaType(part.Header.Get("Content-Type"))
		disposition := part.Header.Get("Content-Disposition")
		isAttachment := strings.HasPrefix(strings.ToLower(disposition), "attachment")
		if cterr != nil || isAttachment {
			continue
		}
		switch {
		case ct == "text/plain" && plain == "":
			data, rerr := readPartDecoded(part.Header, part)
			if rerr != nil {
				return plain, htmlPart, rerr
			}
			text, derr := decodeCharset(params["charset"], data)
			if derr != nil {
				slog.Warn("mail inbound: charset conversion failed, storing raw bytes",
					"error", derr, "charset", params["charset"])
			}
			plain = text
		case ct == "text/html" && htmlPart == "":
			data, rerr := readPartDecoded(part.Header, part)
			if rerr != nil {
				return plain, htmlPart, rerr
			}
			text, derr := decodeCharset(params["charset"], data)
			if derr != nil {
				slog.Warn("mail inbound: charset conversion failed, storing raw bytes",
					"error", derr, "charset", params["charset"])
			}
			htmlPart = text
		case strings.HasPrefix(ct, "multipart/"):
			inner := multipart.NewReader(part, params["boundary"])
			ip, ih, ierr := walkPartsForText(inner)
			if ierr != nil {
				return plain, htmlPart, ierr
			}
			if plain == "" {
				plain = ip
			}
			if htmlPart == "" {
				htmlPart = ih
			}
		}
	}
}

// htmlToText strips HTML markup to readable text: block-level elements become
// line breaks, inline elements concatenate without inserted spaces (a code
// split across adjacent tags must not gain spaces, e.g.
// <span>12</span><span>345</span> → "12345"), entities are unescaped, and
// whitespace is collapsed per line.
func htmlToText(raw string) string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	var b strings.Builder
	htmlTextWalk(doc, &b)
	var lines []string
	for _, line := range strings.Split(b.String(), "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func htmlTextWalk(n *html.Node, b *strings.Builder) {
	if n.Type == html.ElementNode && blockHTMLTags[n.Data] && b.Len() > 0 {
		b.WriteByte('\n')
	}
	if n.Type == html.TextNode {
		b.WriteString(html.UnescapeString(n.Data))
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		htmlTextWalk(c, b)
	}
}

// blockHTMLTags are elements that start a new visual line in rendered email.
var blockHTMLTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"br": true, "dd": true, "div": true, "dl": true, "dt": true,
	"figcaption": true, "figure": true, "footer": true, "form": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "hr": true, "li": true, "main": true, "nav": true,
	"ol": true, "p": true, "pre": true, "section": true, "table": true,
	"tbody": true, "td": true, "th": true, "thead": true, "tr": true, "ul": true,
}

// readPartDecoded reads a multipart part applying its Content-Transfer-Encoding
// (multipart.NewReader does not transfer-decode parts itself — it hands the raw
// bytes back, so a base64 text/html part needs explicit decoding).
func readPartDecoded(header textproto.MIMEHeader, r io.Reader) ([]byte, error) {
	return readTransferDecoded(header.Get("Content-Transfer-Encoding"), r)
}

// readTransferDecoded handles the top-level Content-Transfer-Encoding that
// net/mail does not decode itself.
func readTransferDecoded(cte string, r io.Reader) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(cte)) {
	case "base64":
		return io.ReadAll(base64.NewDecoder(base64.StdEncoding, r))
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(r))
	default:
		return io.ReadAll(r)
	}
}

var wordDecoder = mime.WordDecoder{CharsetReader: charsetReaderForWordDecoder}

func decodeMimeWords(s string) string {
	if s == "" || !strings.Contains(s, "=?") {
		return s
	}
	if decoded, err := wordDecoder.DecodeHeader(s); err == nil {
		return decoded
	}
	return s
}

func charsetReaderForWordDecoder(charset string, input io.Reader) (io.Reader, error) {
	data, err := decodeCharsetBytes(charset, input)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// decodeCharset converts a byte slice in the given charset to UTF-8. Unknown
// charsets fall back to the raw bytes rather than failing the whole message
// (the intake must not lose mail over an exotic charset).
func decodeCharset(charset string, data []byte) (string, error) {
	decoded, err := decodeCharsetBytes(charset, bytes.NewReader(data))
	if err != nil {
		return string(data), err
	}
	return string(decoded), nil
}

func decodeCharsetBytes(charset string, input io.Reader) ([]byte, error) {
	cs := strings.ToLower(strings.TrimSpace(charset))
	if cs == "" || cs == "utf-8" || cs == "utf8" || cs == "us-ascii" || cs == "ascii" {
		return io.ReadAll(input)
	}
	e, err := ianaindex.MIME.Encoding(cs)
	if err != nil || e == nil {
		e, err = ianaindex.IANA.Encoding(cs)
	}
	if err != nil || e == nil {
		return io.ReadAll(input)
	}
	return io.ReadAll(e.NewDecoder().Reader(input))
}

// firstAddress pulls the first addr-spec out of an address header value,
// falling back to the raw trimmed string if it doesn't parse cleanly.
func firstAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if list, err := mail.ParseAddressList(value); err == nil && len(list) > 0 {
		return list[0].Address
	}
	if addr, err := mail.ParseAddress(value); err == nil {
		return addr.Address
	}
	return value
}

// newMailID generates the 12-char hex id the schema (and mailsink) expect.
func newMailID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is effectively impossible; fall back to time.
		return hex.EncodeToString([]byte(time.Now().Format("0102150405")))[:12]
	}
	return hex.EncodeToString(buf)
}

// writeMailJSON writes the plain (non-envelope) JSON responses the external
// mail contract uses — unlike the internal API's {data,error} wrapper.
func writeMailJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseMailLimit(raw string) int {
	const def, max = 20, 200
	if raw == "" {
		return def
	}
	n := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
		if n > max {
			return max
		}
	}
	if n <= 0 {
		return def
	}
	return n
}
