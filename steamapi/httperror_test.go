package steamapi

import (
	"regexp"
	"strings"
	"testing"
)

func TestErrorBodySnippet_PlainShortBody(t *testing.T) {
	got := errorBodySnippet([]byte("Access Denied"))
	if got != "Access Denied" {
		t.Errorf("got %q; want %q unchanged", got, "Access Denied")
	}
}

func TestErrorBodySnippet_LongPlainBodyTruncated(t *testing.T) {
	body := strings.Repeat("x", 400)
	got := errorBodySnippet([]byte(body))
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Errorf("got %q; want a truncation marker suffix", got)
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("got %q; want a single line", got)
	}
	if len(got) > maxErrorBodyLen+len("...(truncated)") {
		t.Errorf("got len %d; want <= %d", len(got), maxErrorBodyLen+len("...(truncated)"))
	}
}

func TestErrorBodySnippet_WhitespaceCollapsed(t *testing.T) {
	got := errorBodySnippet([]byte("  rate\n\tlimit   exceeded  "))
	if got != "rate limit exceeded" {
		t.Errorf("got %q; want %q", got, "rate limit exceeded")
	}
}

func TestErrorBodySnippet_HTMLOmitted(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
<link href="https://community.steamstatic.com/public/css/globalv2.css" rel="stylesheet">
<link href="https://community.steamstatic.com/public/css/fatalerror.css" rel="stylesheet">
</head>
<body>
There was an error communicating with the network. Please try again later.
</body>
</html>`
	got := errorBodySnippet([]byte(html))

	if regexp.MustCompile(`(?i)fatal`).MatchString(got) {
		t.Errorf("got %q; must not contain %q (pollutes audit fatal grep)", got, "fatal")
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("got %q; want a single line", got)
	}
	if len(got) >= 80 {
		t.Errorf("got len %d (%q); want a compact marker (< 80)", len(got), got)
	}
}

func TestErrorBodySnippet_Empty(t *testing.T) {
	if got := errorBodySnippet(nil); got != "" {
		t.Errorf("got %q; want empty string", got)
	}
}

func TestHTTPStatusError_HTMLBody(t *testing.T) {
	html := []byte("<!DOCTYPE html><html><head></head><body>boom</body></html>")
	err := HTTPStatusError(503, html)
	want := "HTTP 503: [html response omitted, 58 bytes]"
	if err.Error() != want {
		t.Errorf("got %q; want %q", err.Error(), want)
	}
}
