package steamapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/k64z/rq"
	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

type RSAPublicKey struct {
	Mod       string
	Exp       int64
	Timestamp uint64
}

// Fetches RSA public key to use to encrypt passwords for a given account name
func GetPasswordRSAPublicKey(ctx context.Context, accountName string) (*RSAPublicKey, error) {
	msg := &protocol.CAuthentication_GetPasswordRSAPublicKey_Request{
		AccountName: &accountName,
	}

	payload, err := encodeProto(msg)
	if err != nil {
		return nil, err
	}

	resp := rq.New().
		URL("https://api.steampowered.com/IAuthenticationService/GetPasswordRSAPublicKey/v1").
		QueryParam("origin", "https://steamcommunity.com").
		QueryParam("input_protobuf_encoded", payload).
		DoContext(ctx)

	result, err := decodeProto(resp, &protocol.CAuthentication_GetPasswordRSAPublicKey_Response{})
	if err != nil {
		return nil, err
	}

	if result.PublickeyMod == nil || result.PublickeyExp == nil {
		return nil, fmt.Errorf("malformed RSA key: %+v", result)
	}

	exp, err := strconv.ParseInt(*result.PublickeyExp, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("parse exp: %w", err)
	}

	return &RSAPublicKey{
		Mod:       *result.PublickeyMod,
		Exp:       exp,
		Timestamp: *result.Timestamp,
	}, nil
}

type SessionPersistence = int32

const (
	SessionPersistenceInvalid SessionPersistence = iota - 1
	SessionPersistenceEphemeral
	SessionPersistencePersistent
)

type PlatformType int32

const (
	PlatformTypeUnknown PlatformType = iota
	PlatformTypeSteamClient
	PlatformTypeWebBrowser
	PlatformTypeMobileApp
)

type DeviceDetails struct {
	FriendlyName string // INFO: user-agent in browser
	PlatformType int32  // TODO: use proper type
}

// BeginAuthSessionWithCredentialsRequest
// TODO: these are only the fields that are during the browser login. Other variant can contain more
type BeginAuthSessionWithCredentialsRequest struct {
	AccountName         string
	EncryptedPassword   string
	EncryptionTimestamp uint64 // NOTE: apparently this is the one which is returned by GetPasswordRSAPublicKey
	RememberLogin       bool   // NOTE: marked as deprecated at https://steamapi.xpaw.me/#IAuthenticationService
	Persistence         int32  // TODO: use proper type
	WebsiteID           string // NOTE: Store when logging from a store page
	DeviceDetails       DeviceDetails
	Language            uint32 // NOTE: English is 0, apparently
}

// SteamSession module could offer some higher level abstractions, like
// - 'Just login as if in browser'
//

func BeginAuthSessionViaCredentials(
	ctx context.Context,
	req *protocol.CAuthentication_BeginAuthSessionViaCredentials_Request,
) (*protocol.CAuthentication_BeginAuthSessionViaCredentials_Response, error) {
	if req == nil {
		return nil, errors.New("invalid request")
	}

	payload, err := encodeProto(req)
	if err != nil {
		return nil, fmt.Errorf("encode proto: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	err = w.WriteField("input_protobuf_encoded", payload)
	if err != nil {
		return nil, fmt.Errorf("write field: %w", err)
	}

	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	resp := rq.New().
		URL("https://api.steampowered.com/IAuthenticationService/BeginAuthSessionViaCredentials/v1").
		Method(http.MethodPost).
		BodyBytes(buf.Bytes()).
		Header("Content-Type", w.FormDataContentType()).
		DoContext(ctx)

	result, err := decodeProto(resp, &protocol.CAuthentication_BeginAuthSessionViaCredentials_Response{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Encodes protobuf messages to base64
func encodeProto(msg proto.Message) (string, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("proto marshal: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// Decodes HTTP responses to protobuf messages
func decodeProto[T proto.Message](resp *rq.Response, msg T) (T, error) {
	if resp.Error() != nil {
		return msg, fmt.Errorf("rq: %w", resp.Error())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return msg, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := resp.Bytes()
	if err != nil {
		return msg, fmt.Errorf("read body: %w", err)
	}

	err = proto.Unmarshal(bodyBytes, msg)
	if err != nil {
		return msg, fmt.Errorf("unmarshal proto: %w", err)
	}

	return msg, nil
}
