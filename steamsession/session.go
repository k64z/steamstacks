package steamsession

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamapi"
	"github.com/k64z/steamstacks/steamid"
	"google.golang.org/protobuf/proto"
)

type Session struct {
	accountName string
	password    string
	steamID     steamid.SteamID

	accessToken  string
	refreshToken string
	clientID     uint64
	requestID    []byte

	httpClient *http.Client

	platformType  protocol.EAuthTokenPlatformType
	defaultHeader http.Header
	websiteID     string // NOTE: PlatformTypeWebBrowser only
	userAgent     string // NOTE: PlatformTypeMobileApp doesn't use it
	language      uint32 // TODO: figure out what codes are these

	pollingStartTime time.Time
	pollingInterval  time.Duration
}

func New(accountName, password string) *Session {
	s := &Session{
		accountName:  accountName,
		password:     password,
		httpClient:   http.DefaultClient,
		platformType: protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_WebBrowser,
		language:     0,
	}

	s.SetHeaders()

	return s
}

// StartWithCredentials
func (s *Session) StartWithCredentials(ctx context.Context) error {
	log.Println("starting authentication session...")
	rsaKey, err := steamapi.GetPasswordRSAPublicKey(ctx, s.accountName)
	if err != nil {
		return fmt.Errorf("get RSA public key: %w", err)
	}

	encryptedPassword, err := encryptPassword(s.password, rsaKey.Mod, rsaKey.Exp)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}

	req := &protocol.CAuthentication_BeginAuthSessionViaCredentials_Request{
		AccountName:         &s.accountName,
		EncryptedPassword:   &encryptedPassword,
		EncryptionTimestamp: &rsaKey.Timestamp,
		RememberLogin:       proto.Bool(true),
		Persistence:         protocol.ESessionPersistence_k_ESessionPersistence_Persistent.Enum(),
		WebsiteId:           &s.websiteID,
		DeviceDetails: &protocol.CAuthentication_DeviceDetails{
			DeviceFriendlyName: &s.userAgent,
			PlatformType:       &s.platformType,
		},
		Language: &s.language,
	}

	authSession, err := steamapi.BeginAuthSessionViaCredentials(ctx, req)
	if err != nil {
		return fmt.Errorf("begin session: %w", err)
	}

	log.Println("authentication session started successfully")

	s.pollingInterval = time.Duration(*authSession.Interval * float32(time.Second))

	return nil
}
