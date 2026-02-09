package steamsession

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamapi"
	"github.com/k64z/steamstacks/steamid"
	"google.golang.org/protobuf/proto"
)

var (
	ErrEmptyUsername = errors.New("account name cannot be empty")
	ErrEmptyPassword = errors.New("password cannot be empty")
)

type Session struct {
	httpClient *http.Client
	steamAPI   *steamapi.API

	AccessToken  string
	RefreshToken string

	SteamID   steamid.SteamID
	weakToken string // INFO: short-lived handle required to finalize the Steam web login flow
	sessionID string
	clientID  uint64 // INFO: something like '3737697558462176538'. Used for SteamGuard and polling
	requestID []byte // INFO: 128 bit blob. Looks like only used for polling

	platformType protocol.EAuthTokenPlatformType
	persistence  protocol.ESessionPersistence
	websiteID    string // NOTE: PlatformTypeWebBrowser only
	userAgent    string // NOTE: PlatformTypeMobileApp doesn't use it
	language     uint32 // TODO: figure out what codes are these

	pollingInterval time.Duration // INFO: returned by 'BeginAuthSession...', usually 5 seconds
}

type config struct {
	httpClient *http.Client
}

type Option func(options *config) error

func WithHTTPClient(httpClient *http.Client) Option {
	return func(options *config) error {
		if httpClient == nil {
			return errors.New("httpClient should be non-nil")
		}
		options.httpClient = httpClient
		return nil
	}
}

func New(opts ...Option) (*Session, error) {
	var cfg config
	for _, opt := range opts {
		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	s := &Session{
		platformType: protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_WebBrowser,
		persistence:  protocol.ESessionPersistence_k_ESessionPersistence_Persistent,
		language:     DefaultLanguageCode,
	}

	if cfg.httpClient != nil {
		s.httpClient = cfg.httpClient
	} else {
		s.httpClient = http.DefaultClient
	}

	// Ensure the HTTP client has a cookie jar for web authentication
	if s.httpClient.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("create cookie jar: %w", err)
		}
		s.httpClient.Jar = jar
	}

	var err error
	s.steamAPI, err = steamapi.New(steamapi.WithHTTPClient(s.httpClient))
	if err != nil {
		return nil, fmt.Errorf("init SteamAPI: %w", err)
	}

	s.SetHeaders()

	return s, nil
}

// LoginWithDeviceCode is a convenience method that performs the most common workflow
func (s *Session) LoginWithDeviceCode(ctx context.Context, username, password, code string) error {
	guardTypes, err := s.StartWithCredentials(ctx, username, password)
	if err != nil {
		return fmt.Errorf("start with credentials: %w", err)
	}

	if !slices.Contains(guardTypes, EAuthSessionGuardTypeDeviceCode) {
		return errors.New("device code authentication is not allowed")
	}

	err = s.SubmitSteamGuardCode(ctx, code, EAuthSessionGuardTypeDeviceCode)
	if err != nil {
		return fmt.Errorf("submit steam guard code: %w", err)
	}

	err = s.PollAuthSessionStatus(ctx)
	if err != nil {
		return fmt.Errorf("poll auth session status: %w", err)
	}

	return nil
}

// StartWithCredentials
func (s *Session) StartWithCredentials(ctx context.Context, username, password string) ([]EAuthSessionGuardType, error) {
	if strings.TrimSpace(username) == "" {
		return nil, ErrEmptyUsername
	}

	if strings.TrimSpace(password) == "" {
		return nil, ErrEmptyPassword
	}
	log.Println("starting authentication session...")
	rsaKey, err := s.steamAPI.GetPasswordRSAPublicKey(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get RSA public key: %w", err)
	}

	encryptedPassword, err := encryptPassword(password, rsaKey.Mod, rsaKey.Exp)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}

	req := &protocol.CAuthentication_BeginAuthSessionViaCredentials_Request{
		AccountName:         &username,
		EncryptedPassword:   &encryptedPassword,
		EncryptionTimestamp: &rsaKey.Timestamp,
		RememberLogin:       proto.Bool(true),
		Persistence:         &s.persistence,
		WebsiteId:           &s.websiteID,
		DeviceDetails: &protocol.CAuthentication_DeviceDetails{
			DeviceFriendlyName: &s.userAgent,
			PlatformType:       &s.platformType,
		},
		Language: &s.language,
	}

	authSession, err := s.steamAPI.BeginAuthSessionViaCredentials(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("begin session: %w", err)
	}

	s.clientID = *authSession.ClientId
	s.requestID = authSession.RequestId
	s.pollingInterval = time.Duration(*authSession.Interval * float32(time.Second))
	s.SteamID = steamid.FromSteamID64(*authSession.Steamid)
	s.weakToken = *authSession.WeakToken

	guardTypes := []EAuthSessionGuardType{}
	for _, conf := range authSession.AllowedConfirmations {
		switch *conf.ConfirmationType {
		case protocol.EAuthSessionGuardType_k_EAuthSessionGuardType_Unknown:
			guardTypes = append(guardTypes, EAuthSessionGuardTypeUnknown)
		case protocol.EAuthSessionGuardType_k_EAuthSessionGuardType_None:
			guardTypes = append(guardTypes, EAuthSessionGuardTypeNone)
		case protocol.EAuthSessionGuardType_k_EAuthSessionGuardType_EmailCode:
			guardTypes = append(guardTypes, EAuthSessionGuardTypeEmailCode)
		case protocol.EAuthSessionGuardType_k_EAuthSessionGuardType_DeviceCode:
			guardTypes = append(guardTypes, EAuthSessionGuardTypeDeviceCode)
		case protocol.EAuthSessionGuardType_k_EAuthSessionGuardType_DeviceConfirmation:
			guardTypes = append(guardTypes, EAuthSessionGuardTypeDeviceConfirmation)
		case protocol.EAuthSessionGuardType_k_EAuthSessionGuardType_EmailConfirmation:
			guardTypes = append(guardTypes, EAuthSessionGuardTypeEmailConfirmation)
		case protocol.EAuthSessionGuardType_k_EAuthSessionGuardType_MachineToken:
			guardTypes = append(guardTypes, EAuthSessionGuardTypeMachineToken)
		}
	}

	log.Println("authentication session started successfully")
	return guardTypes, nil
}

// SubmitSteamGuardCode approves a session with Steam Guard code
// If this method returns no error, polling can be started
func (s *Session) SubmitSteamGuardCode(ctx context.Context, code string, guardType EAuthSessionGuardType) error {
	req := &protocol.CAuthentication_UpdateAuthSessionWithSteamGuardCode_Request{
		ClientId: &s.clientID,
		Steamid:  (*uint64)(&s.SteamID),
		Code:     &code,
		CodeType: (*protocol.EAuthSessionGuardType)(&guardType),
	}

	err := s.steamAPI.UpdateAuthSessionWithSteamGuardCode(ctx, req)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return nil
}

func (s *Session) PollAuthSessionStatus(ctx context.Context) error {
	req := &protocol.CAuthentication_PollAuthSessionStatus_Request{
		ClientId:  &s.clientID,
		RequestId: s.requestID,
	}

	for {
		log.Println("polling")

		resp, err := s.steamAPI.PollAuthSessionStatus(ctx, req)
		if err != nil {
			log.Printf("error polling session: %v", err)
			select {
			case <-ctx.Done():
				log.Println("polling cancelled")
				return ctx.Err()
			case <-time.After(s.pollingInterval):
			}
		} else {
			if resp.AccessToken == nil {
				return errors.New("access token is nil")
			}
			if resp.RefreshToken == nil {
				return errors.New("refresh token is nil")
			}
			s.AccessToken = *resp.AccessToken
			s.RefreshToken = *resp.RefreshToken
			return nil
		}
	}
}

type PersistentSession struct {
	RefreshToken string          `json:"refresh_token"`
	SteamID      steamid.SteamID `json:"steam_id"`
}

func (s *Session) SaveToFile(filePath string) error {
	if s.RefreshToken == "" {
		return errors.New("no refresh token to save")
	}

	data := PersistentSession{
		RefreshToken: s.RefreshToken,
		SteamID:      s.SteamID,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal session data: %w", err)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	log.Printf("Session saved to %s", filePath)
	return nil
}

func (s *Session) LoadFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("session file does not exist")
		}
		return fmt.Errorf("read session file: %w", err)
	}

	var persistentSession PersistentSession
	if err := json.Unmarshal(data, &persistentSession); err != nil {
		return fmt.Errorf("unmarshal session data: %w", err)
	}

	s.RefreshToken = persistentSession.RefreshToken
	s.SteamID = persistentSession.SteamID

	log.Printf("Session loaded from %s", filePath)
	return nil
}

func (s *Session) IsValidToken(ctx context.Context) bool {
	if s.RefreshToken == "" {
		return false
	}
	exp, err := jwtExpiry(s.RefreshToken)
	if err != nil {
		return false
	}
	return time.Now().Before(exp)
}

func jwtExpiry(token string) (time.Time, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return time.Time{}, errors.New("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, err
	}
	if claims.Exp == 0 {
		return time.Time{}, errors.New("missing exp claim")
	}
	return time.Unix(claims.Exp, 0), nil
}

// HTTPClient returns the session's underlying HTTP client.
// After authentication and GetWebCookies, its cookie jar holds all session
// state needed to construct steamapi.API or steamcommunity.Community instances.
func (s *Session) HTTPClient() *http.Client {
	return s.httpClient
}

// DoRequest executes an arbitrary HTTP request using the session's httpClient
func (s *Session) DoRequest(req *http.Request) (*http.Response, error) {
	return s.httpClient.Do(req)
}
