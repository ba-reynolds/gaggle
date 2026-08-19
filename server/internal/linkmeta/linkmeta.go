package linkmeta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ba-reynolds/gaggle/internal/models"
	"golang.org/x/net/html"
)

var (
	httpClient = &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	// maxBodyBytes caps how much of the linked page we'll read. OG tags live in
	// <head>, so we never need the whole document.
	maxBodyBytes int64 = 2 * 1024 * 1024

	userAgent = "gaggle-link-previewer/1.0"
)

// Preview fetches rawURL and extracts the OpenGraph metadata (title, image,
// site name) for a news link card. Title falls back to the page <title> tag.
// On any fetch/parse failure it returns the URL-only NewsLink so the caller can
// still persist an attachment (the card degrades to a bare link).
func Preview(ctx context.Context, rawURL string) (*models.NewsLink, error) {
	news := &models.NewsLink{URL: rawURL}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return news, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return news, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			// The client follows redirects; a manual Location means we can't preview.
			return news, fmt.Errorf("unexpected redirect status %d", resp.StatusCode)
		}
		return news, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" &&
		!strings.Contains(strings.ToLower(contentType), "text/html") &&
		!strings.Contains(strings.ToLower(contentType), "application/xhtml+xml") {
		return news, fmt.Errorf("content type %q is not HTML", contentType)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return news, err
	}

	title, image, siteName := parseMeta(body)
	if title == "" {
		title = parseTitleTag(body)
	}
	news.Title = title
	news.ImageURL = resolveImageURL(rawURL, image)
	news.SiteName = siteName
	return news, nil
}

// parseMeta walks the document looking for <meta name="og:*" content="...">
// (both `name` and `property` spellings are seen in the wild).
func parseMeta(doc []byte) (title, image, siteName string) {
	tokenizer := html.NewTokenizer(strings.NewReader(string(doc)))
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return title, image, siteName
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data != "meta" {
				continue
			}
			var key, content string
			for _, attr := range token.Attr {
				if attr.Key == "property" || attr.Key == "name" {
					key = attr.Val
				}
				if attr.Key == "content" {
					content = attr.Val
				}
			}
			switch strings.TrimSpace(strings.ToLower(key)) {
			case "og:title":
				if title == "" && strings.TrimSpace(content) != "" {
					title = strings.TrimSpace(content)
				}
			case "og:image":
				if image == "" && strings.TrimSpace(content) != "" {
					image = strings.TrimSpace(content)
				}
			case "og:site_name":
				if siteName == "" && strings.TrimSpace(content) != "" {
					siteName = strings.TrimSpace(content)
				}
			}
		}
	}
}

func parseTitleTag(doc []byte) string {
	tokenizer := html.NewTokenizer(strings.NewReader(string(doc)))
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return ""
		case html.StartTagToken:
			token := tokenizer.Token()
			if token.Data != "title" {
				continue
			}
			if tt = tokenizer.Next(); tt != html.TextToken {
				return ""
			}
			return strings.TrimSpace(tokenizer.Token().Data)
		}
	}
}

// resolveImageURL makes an absolute URL out of a possibly-relative og:image path.
func resolveImageURL(base, image string) string {
	if image == "" {
		return ""
	}
	if parsed, err := url.Parse(image); err == nil && parsed.IsAbs() {
		return image
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	resolved := baseURL.ResolveReference(&url.URL{Path: image})
	return resolved.String()
}
