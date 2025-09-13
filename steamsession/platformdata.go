package steamsession

import (
	"net/http"
)

const (
	// Browser User Agent for web-based authentication
	BrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

	WebsiteIDClient    = "Client"
	WebsiteIDCommunity = "Community"
	WebsiteIDMobile    = "Mobile"

	// 0 = English/default
	DefaultLanguageCode = uint32(0)
)

func (s *Session) SetHeaders() {
	// These are WebBrowser headers
	if s.defaultHeader == nil {
		s.defaultHeader = make(http.Header)
	}

	s.defaultHeader.Set("User-Agent", BrowserUA)
	s.defaultHeader.Set("Origin", "https://steamcommunity.com")
	s.defaultHeader.Set("Referer", "https://steamcommunity.com")

	s.userAgent = BrowserUA
	s.websiteID = WebsiteIDCommunity
}
