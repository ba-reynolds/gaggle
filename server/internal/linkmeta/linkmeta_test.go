package linkmeta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreviewExtractsOpenGraph(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head>
			<meta property="og:title" content="  Barge captain reunites with lost cat  ">
			<meta name="og:image" content="/deck/captain-and-cat.jpg">
			<meta property="og:site_name" content="The Gaggle Herald">
			<title>Fallback Title</title>
		</head><body>hello</body></html>`))
	}))
	defer srv.Close()

	news, err := Preview(context.Background(), srv.URL+"/articles/barge-cat")
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if want := "Barge captain reunites with lost cat"; news.Title != want {
		t.Fatalf("title = %q, want %q", news.Title, want)
	}
	if want := srv.URL + "/deck/captain-and-cat.jpg"; news.ImageURL != want {
		t.Fatalf("image_url = %q, want %q", news.ImageURL, want)
	}
	if want := "The Gaggle Herald"; news.SiteName != want {
		t.Fatalf("site_name = %q, want %q", news.SiteName, want)
	}
	if news.URL != srv.URL+"/articles/barge-cat" {
		t.Fatalf("url = %q, want the requested URL", news.URL)
	}
}

func TestPreviewFallsBackToTitleTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title> Just the title </title></head></html>`))
	}))
	defer srv.Close()

	news, err := Preview(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if want := "Just the title"; news.Title != want {
		t.Fatalf("title = %q, want %q", news.Title, want)
	}
	if news.ImageURL != "" {
		t.Fatalf("image_url = %q, want empty", news.ImageURL)
	}
}

func TestPreviewKeepsURLOnlyOnFetchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nope": true}`))
	}))
	defer srv.Close()

	news, err := Preview(context.Background(), srv.URL+"/not-html")
	if err == nil {
		t.Fatalf("expected an error for non-HTML content, got nil")
	}
	if news == nil || news.URL != srv.URL+"/not-html" {
		t.Fatalf("URL-only NewsLink not returned: %+v err %v", news, err)
	}
	if news.Title != "" || news.ImageURL != "" {
		t.Fatalf("failed preview should stay bare, got %+v", news)
	}
}

func TestPreviewRejectsTooLargeBodyCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		big := strings.Repeat("a", int(maxBodyBytes)+1024)
		_, _ = w.Write([]byte(big))
	}))
	defer srv.Close()

	news, err := Preview(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("oversized page Preview: %v", err)
	}
	if news == nil || news.URL != srv.URL {
		t.Fatalf("URL-only NewsLink not returned: %+v", news)
	}
}
