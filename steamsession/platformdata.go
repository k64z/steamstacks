package steamsession

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

	s.userAgent = BrowserUA
	s.websiteID = WebsiteIDCommunity
}
