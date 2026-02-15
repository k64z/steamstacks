package steamsession

import "github.com/k64z/steamstacks/protocol"

const (
	// Browser User Agent for web-based authentication
	BrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

	// SteamClientUA mimics the official Steam client's user-agent string.
	SteamClientUA = "Valve/Steam HTTP Client 1.0"

	WebsiteIDClient    = "Client"
	WebsiteIDCommunity = "Community"
	WebsiteIDMobile    = "Mobile"

	// 0 = English/default
	DefaultLanguageCode = uint32(0)
)

func (s *Session) SetHeaders() {
	switch s.platformType {
	case protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_SteamClient:
		s.userAgent = SteamClientUA
		s.websiteID = WebsiteIDClient
	case protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_MobileApp:
		s.userAgent = BrowserUA
		s.websiteID = WebsiteIDMobile
	default:
		s.userAgent = BrowserUA
		s.websiteID = WebsiteIDCommunity
	}
}
