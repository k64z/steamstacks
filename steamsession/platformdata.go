package steamsession

import (
	"net/http"
)

const BrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

const (
	WebsiteIDClient    = "Client"
	WebsiteIDCommunity = "Community"
	WebsiteIDMobile    = "Mobile"
)

func (s *Session) SetHeaders() {
	// These are WebBroweser headers
	if s.defaultHeader == nil {
		s.defaultHeader = make(http.Header)
	}

	s.defaultHeader.Set("User-Agent", BrowserUA)
	s.defaultHeader.Set("Origin", "https://steamcommunity.com")
	s.defaultHeader.Set("Referer", "https://steamcommunity.com")

	s.userAgent = BrowserUA
	s.websiteID = "Store"
}
