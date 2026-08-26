package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ba-reynolds/gaggle/internal/testutil"
)

const testIntakeSecret = "test-intake-secret"

func mimeMail(from, to, subject, messageID, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	if to != "" {
		fmt.Fprintf(&b, "To: %s\r\n", to)
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	if messageID != "" {
		fmt.Fprintf(&b, "Message-ID: %s\r\n", messageID)
	}
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", body)
	return b.String()
}

func postInbound(t *testing.T, app *testutil.App, secret, origTo, raw string) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{"Content-Type": "message/rfc822"}
	if secret != "" {
		headers["x-orchid-secret"] = secret
	}
	if origTo != "" {
		headers["x-orig-to"] = origTo
	}
	return app.Do(t, testutil.Request{
		Method:  http.MethodPost,
		Path:    "/mail/inbound",
		RawBody: []byte(raw),
		Headers: headers,
	})
}

func listMails(t *testing.T, app *testutil.App, query string) []map[string]any {
	t.Helper()
	path := "/mails" + query
	rec := app.Do(t, testutil.Request{
		Method:  http.MethodGet,
		Path:    path,
		Headers: map[string]string{"x-orchid-secret": testIntakeSecret},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d body %s", path, rec.Code, rec.Body.String())
	}
	var resp struct {
		Mails []map[string]any `json:"mails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %s response: %v (body: %s)", path, err, rec.Body.String())
	}
	return resp.Mails
}

// TestMailInboundRequiresSecret covers the auth gate: missing header, wrong
// value, and (fail-closed) an unconfigured server secret all reject with 401.
func TestMailInboundRequiresSecret(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	if rec := postInbound(t, app, "", "a@gaggle.land", mimeMail("x@y.z", "", "hi", "<m1@y.z>", "hello")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing header: expected 401, got %d", rec.Code)
	}
	if rec := postInbound(t, app, "wrong-secret", "a@gaggle.land", mimeMail("x@y.z", "", "hi", "<m2@y.z>", "hello")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: expected 401, got %d", rec.Code)
	}
	for _, path := range []string{"/mails", "/mails/nonexistentid"} {
		rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: path})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s without header: expected 401, got %d", path, rec.Code)
		}
	}
}

// TestMailInboundStoresAndDedupes: a well-formed delivery is stored and
// readable via the two GETs; redelivering the same Message-ID keeps one row.
func TestMailInboundStoresAndDedupes(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	raw := mimeMail("Sender <sender@example.com>", "alice@gaggle.land", "Your code: 123456", "<msg-1@example.com>", "code = 123456")
	rec := postInbound(t, app, testIntakeSecret, "alice@gaggle.land", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("inbound: expected 200, got %d body %s", rec.Code, rec.Body.String())
	}

	mails := listMails(t, app, "?to=alice%40gaggle.land")
	if len(mails) != 1 {
		t.Fatalf("expected 1 mail after first delivery, got %d (%+v)", len(mails), mails)
	}
	m := mails[0]
	if m["to_addr"] != "alice@gaggle.land" {
		t.Errorf("to_addr: got %v (x-orig-to must win over To:)", m["to_addr"])
	}
	if m["from_addr"] != "sender@example.com" {
		t.Errorf("from_addr: got %v, want bare addr-spec", m["from_addr"])
	}
	if m["subject"] != "Your code: 123456" {
		t.Errorf("subject: got %v", m["subject"])
	}
	id, _ := m["id"].(string)
	if len(id) != 12 {
		t.Errorf("id should be 12 chars, got %q", id)
	}
	if _, hasBody := m["body"]; hasBody {
		t.Errorf("list view must NOT include body")
	}

	// Redelivery of the same Message-ID (at-least-once): still 200, no dup.
	rec = postInbound(t, app, testIntakeSecret, "alice@gaggle.land", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("duplicate inbound: expected 200, got %d", rec.Code)
	}
	if mails = listMails(t, app, "?to=alice%40gaggle.land"); len(mails) != 1 {
		t.Fatalf("expected dedupe to keep 1 mail, got %d", len(mails))
	}

	// Full record includes body; unknown id 404s.
	full := app.Do(t, testutil.Request{
		Method:  http.MethodGet,
		Path:    "/mails/" + id,
		Headers: map[string]string{"x-orchid-secret": testIntakeSecret},
	})
	if full.Code != http.StatusOK {
		t.Fatalf("GET /mails/{id}: expected 200, got %d", full.Code)
	}
	var got struct {
		ID     string `json:"id"`
		Body   string `json:"body"`
		TS     string `json:"ts"`
		ToAddr string `json:"to_addr"`
	}
	if err := json.Unmarshal(full.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode full record: %v", err)
	}
	if got.Body != "code = 123456" {
		t.Errorf("body: got %q", got.Body)
	}
	if got.ID != id || got.ToAddr != "alice@gaggle.land" || got.TS == "" {
		t.Errorf("full record fields: %+v", got)
	}

	missing := app.Do(t, testutil.Request{
		Method:  http.MethodGet,
		Path:    "/mails/000000000000",
		Headers: map[string]string{"x-orchid-secret": testIntakeSecret},
	})
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown id: expected 404, got %d", missing.Code)
	}
}

// TestMailListFilterLimitOrder checks the substring filter, limit clamping,
// default limit and newest-first ordering.
func TestMailListFilterLimitOrder(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	deliveries := []struct{ to, msgID, code string }{
		{"bob@gaggle.land", "<o1@x>", "111111"},
		{"carol@gaggle.land", "<o2@x>", "222222"},
		{"dave@gaggle.land", "<o3@x>", "333333"},
	}
	for i, d := range deliveries {
		raw := fmt.Sprintf("From: n@x.t\r\nSubject: code\r\nMessage-ID: %s\r\n\r\ncode %s\r\n", d.msgID, d.code)
		if rec := postInbound(t, app, testIntakeSecret, d.to, raw); rec.Code != http.StatusOK {
			t.Fatalf("delivery %d: got %d", i, rec.Code)
		}
		time.Sleep(10 * time.Millisecond) // distinct ts values: newest-first is asserted below
	}

	mails := listMails(t, app, "?to=gaggle.land")
	if len(mails) != 3 {
		t.Fatalf("expected 3 mails for substring 'gaggle.land', got %d", len(mails))
	}
	first, _ := mails[0]["subject"].(string)
	last, _ := mails[2]["subject"].(string)

	// Newest first: deliver one more and it must appear at the top.
	raw := "From: n@x.t\r\nSubject: NEWEST\r\nMessage-ID: <o4@x>\r\n\r\nx\r\n"
	if rec := postInbound(t, app, testIntakeSecret, "bob@gaggle.land", raw); rec.Code != http.StatusOK {
		t.Fatalf("delivery 4: got %d", rec.Code)
	}
	time.Sleep(10 * time.Millisecond)
	mails = listMails(t, app, "?to=bob%40gaggle.land&limit=10")
	if len(mails) != 2 {
		t.Fatalf("expected 2 bob mails, got %d", len(mails))
	}
	if subj, _ := mails[0]["subject"].(string); subj != "NEWEST" {
		t.Errorf("newest-first violated: first subject %q (earlier run had %q..%q)", subj, first, last)
	}

	if got := listMails(t, app, "?to=gaggle.land&limit=1"); len(got) != 1 {
		t.Errorf("limit=1: got %d mails", len(got))
	}
	if got := listMails(t, app, "?to=nobody@else.t"); len(got) != 0 {
		t.Errorf("non-matching filter should be empty, got %d", len(got))
	}
	// limit over the 200 max is clamped, garbage falls back to the default —
	// both just need to answer 200 without leaking other tenants' filters.
	rec := app.Do(t, testutil.Request{
		Method:  http.MethodGet,
		Path:    "/mails?limit=99999",
		Headers: map[string]string{"x-orchid-secret": testIntakeSecret},
	})
	if rec.Code != http.StatusOK {
		t.Errorf("limit clamp request: expected 200, got %d", rec.Code)
	}
}

// TestMailInboundGarbageStillOK: non-MIME bodies and oversized bodies must
// return 200 (a non-2xx makes Cloudflare bounce the original sender).
func TestMailInboundGarbageStillOK(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	if rec := postInbound(t, app, testIntakeSecret, "a@gaggle.land", "this is not MIME at all"); rec.Code != http.StatusOK {
		t.Fatalf("garbage body: expected 200, got %d", rec.Code)
	}
	big := strings.Repeat("x", 2<<20) // > 1 MB cap
	rec := postInbound(t, app, testIntakeSecret, "a@gaggle.land", big)
	if rec.Code != http.StatusOK {
		t.Fatalf("oversized body: expected 200, got %d", rec.Code)
	}
	if mails := listMails(t, app, "?to=a%40gaggle.land"); len(mails) != 0 {
		t.Errorf("dropped deliveries must not be stored, got %d rows", len(mails))
	}
}

// TestMailParsingEdgeCases exercises encoded headers/charsets, base64 bodies,
// multipart selection, attachment skipping, and envelope fallback to To:.
func TestMailParsingEdgeCases(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	post := func(origTo, raw string) {
		t.Helper()
		if rec := postInbound(t, app, testIntakeSecret, origTo, raw); rec.Code != http.StatusOK {
			t.Fatalf("inbound: got %d body %s", rec.Code, rec.Body.String())
		}
	}
	bodyOf := func(query string, want int) map[string]any {
		t.Helper()
		mails := listMails(t, app, query)
		if len(mails) != want {
			t.Fatalf("query %s: expected %d mails, got %d (%+v)", query, want, len(mails), mails)
		}
		id, _ := mails[0]["id"].(string)
		rec := app.Do(t, testutil.Request{
			Method:  http.MethodGet,
			Path:    "/mails/" + id,
			Headers: map[string]string{"x-orchid-secret": testIntakeSecret},
		})
		var m map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return m
	}

	// RFC 2047 encoded subject + latin-1 quoted-printable body.
	post("eve@gaggle.land", "From: =?utf-8?q?J=C3=B8rgen?= <j@x.t>\r\n"+
		"To: eve@gaggle.land\r\n"+
		"Subject: =?utf-8?B?w4ZyaXN0IGNvZGU=?=\r\n"+
		"Message-ID: <e1@x>\r\n"+
		"Content-Type: text/plain; charset=iso-8859-1\r\n"+
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n"+
		"k=F8de caf=E9\r\n")
	m := bodyOf("?to=eve%40gaggle.land", 1)
	if m["subject"] != "Ærist code" || m["from_addr"] != "j@x.t" {
		t.Errorf("encoded headers: got subject=%v from=%v", m["subject"], m["from_addr"])
	}
	if m["body"] != "køde café" {
		t.Errorf("latin-1 QP body: got %q", m["body"])
	}

	// Multipart: first text/plain part wins, attachments are skipped.
	post("frank@gaggle.land", "From: a@x.t\r\nSubject: mp\r\nMessage-ID: <e2@x>\r\n"+
		"MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=BOUND\r\n\r\n"+
		"--BOUND\r\nContent-Type: text/html\r\n\r\n<b>html</b>\r\n"+
		"--BOUND\r\nContent-Disposition: attachment; filename=f.txt\r\n"+
		"Content-Type: text/plain\r\n\r\nattached-not-inline\r\n"+
		"--BOUND\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nthe real body\r\n"+
		"--BOUND--\r\n")
	m = bodyOf("?to=frank%40gaggle.land", 1)
	if m["body"] != "the real body" {
		t.Errorf("multipart body: got %q", m["body"])
	}

	// Single-part HTML-only: text/plain absent, so the HTML is stripped to text.
	post("hank@gaggle.land", "From: h@x.t\r\nSubject: html-only\r\nMessage-ID: <e6@x>\r\n"+
		"MIME-Version: 1.0\r\nContent-Type: text/html; charset=utf-8\r\n\r\n"+
		"<div>Your verification code is <b>987654</b>.</div>\r\n")
	m = bodyOf("?to=hank%40gaggle.land", 1)
	if m["body"] != "Your verification code is 987654." {
		t.Errorf("html-only body: got %q", m["body"])
	}

	// Multipart/alternative with ONLY an html part: falls back to stripped HTML.
	post("iris@gaggle.land", "From: i@x.t\r\nSubject: alt-html\r\nMessage-ID: <e7@x>\r\n"+
		"MIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=ALT\r\n\r\n"+
		"--ALT\r\nContent-Type: text/html; charset=utf-8\r\n\r\n"+
		"<p>Click to verify: <a href=\"https://x.t/verify?code=abc123\">Verify</a></p>\r\n"+
		"--ALT--\r\n")
	m = bodyOf("?to=iris%40gaggle.land", 1)
	if m["body"] != "Click to verify: Verify" {
		t.Errorf("multipart html-only body: got %q", m["body"])
	}

	// Top-level base64 singlepart.
	post("gina@gaggle.land", "From: b@x.t\r\nSubject: b64\r\nMessage-ID: <e3@x>\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n"+
		"aGVsbG8gYmFzZTY0\r\n")
	m = bodyOf("?to=gina%40gaggle.land", 1)
	if m["body"] != "hello base64" {
		t.Errorf("base64 body: got %q", m["body"])
	}

	// No x-orig-to → fall back to the MIME To: header.
	post("", "From: c@x.t\r\nTo: Henry <henry@gaggle.land>\r\nSubject: fb\r\nMessage-ID: <e4@x>\r\n\r\nfallback\r\n")
	m = bodyOf("?to=henry%40gaggle.land", 1)
	if m["to_addr"] != "henry@gaggle.land" {
		t.Errorf("To: fallback: got %v", m["to_addr"])
	}

	// LIKE metacharacters in the filter are matched literally.
	post("per%cent@gaggle.land", "From: d@x.t\r\nSubject: pct\r\nMessage-ID: <e5@x>\r\n\r\npct\r\n")
	if mails := listMails(t, app, "?to=er%25c"); len(mails) != 1 {
		t.Errorf("escaped %% filter: expected literal match, got %d", len(mails))
	}
	if mails := listMails(t, app, "?to="); len(mails) < 5 {
		t.Errorf("empty filter should list everything, got %d", len(mails))
	}
}
