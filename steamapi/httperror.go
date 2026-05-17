package steamapi

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// maxErrorBodyLen caps the number of bytes of a (non-HTML) response body that
// errorBodySnippet embeds into an error string. Steam often answers with
// multi-KB HTML error pages on transient 5xx; without a cap those land
// verbatim in a single journald line.
const maxErrorBodyLen = 256

// htmlOpenTags are the tag-open forms that mark a body as an HTML document.
// The trailing `>`/space is required so a JSON error echoing user input like
// `"<body"` isn't misclassified as HTML and silently discarded.
var htmlOpenTags = []string{"<head>", "<head ", "<body>", "<body "}

// errorBodySnippet returns a log-safe one-line summary of an HTTP response
// body for embedding in error strings.
//
// Steam serves HTML error pages (e.g. "There was an error communicating with
// the network") on transient 5xx instead of JSON. Those pages are several KB
// and contain "fatalerror.css", which pollutes log greps for the word
// "fatal". For an HTML body we therefore drop the content entirely and report
// only its size; for any other body we collapse whitespace and cap the length.
func errorBodySnippet(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	probe := trimmed
	if len(probe) > 512 {
		probe = probe[:512]
	}
	lower := strings.ToLower(probe)
	if strings.HasPrefix(lower, "<!doctype") || strings.HasPrefix(lower, "<html") ||
		containsAny(lower, htmlOpenTags) {
		return fmt.Sprintf("[html response omitted, %d bytes]", len(body))
	}

	snippet := strings.Join(strings.Fields(trimmed), " ")
	if len(snippet) > maxErrorBodyLen {
		cut := maxErrorBodyLen
		for cut > 0 && !utf8.RuneStart(snippet[cut]) {
			cut--
		}
		snippet = snippet[:cut] + "...(truncated)"
	}
	return snippet
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// HTTPStatusError builds the canonical "HTTP <code>: <body>" error used across
// the Steam clients, with the body run through errorBodySnippet so no call
// site can leak a multi-KB HTML page into the logs.
func HTTPStatusError(status int, body []byte) error {
	return fmt.Errorf("HTTP %d: %s", status, errorBodySnippet(body))
}
