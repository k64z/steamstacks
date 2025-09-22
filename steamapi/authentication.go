package steamapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"strconv"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

type RSAPublicKey struct {
	Mod       string
	Exp       int64
	Timestamp uint64
}

// Fetches RSA public key to use to encrypt passwords for a given account name
func (a *API) GetPasswordRSAPublicKey(ctx context.Context, accountName string) (*RSAPublicKey, error) {
	msg := &protocol.CAuthentication_GetPasswordRSAPublicKey_Request{
		AccountName: &accountName,
	}

	payload, err := encodeProto(msg)
	if err != nil {
		return nil, err
	}

	apiURL := "https://api.steampowered.com/IAuthenticationService/GetPasswordRSAPublicKey/v1"
	params := url.Values{}
	params.Set("origin", "https://steamcommunity.com")
	params.Set("input_protobuf_encoded", payload)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Eresult") != "1" {
		return nil, fmt.Errorf("invalid X-Eresult header: %s", resp.Header.Get("X-Eresult"))
	}

	result, err := decodeProtoFromHTTPResponse(resp, &protocol.CAuthentication_GetPasswordRSAPublicKey_Response{})
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

func (a *API) BeginAuthSessionViaCredentials(
	ctx context.Context,
	req *protocol.CAuthentication_BeginAuthSessionViaCredentials_Request,
) (*protocol.CAuthentication_BeginAuthSessionViaCredentials_Response, error) {
	if req == nil {
		return nil, errors.New("invalid request")
	}

	bodyBytes, contentType, err := buildProtobufPOSTBody(req)
	if err != nil {
		return nil, fmt.Errorf("build body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.steampowered.com/IAuthenticationService/BeginAuthSessionViaCredentials/v1", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", contentType)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Eresult") != "1" {
		return nil, fmt.Errorf("invalid X-Eresult header: %s", resp.Header.Get("X-Eresult"))
	}
	if resp.Header.Get("Content-Type") != "application/octet-stream" {
		return nil, fmt.Errorf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}

	result, err := decodeProtoFromHTTPResponse(resp, &protocol.CAuthentication_BeginAuthSessionViaCredentials_Response{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UpdateAuthSessionWithSteamGuardCode approves an authentication session via steam guard code
func (a *API) UpdateAuthSessionWithSteamGuardCode(
	ctx context.Context,
	req *protocol.CAuthentication_UpdateAuthSessionWithSteamGuardCode_Request,
) error {
	if req == nil {
		return errors.New("invalid request")
	}

	bodyBytes, contentType, err := buildProtobufPOSTBody(req)
	if err != nil {
		return fmt.Errorf("build body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.steampowered.com/IAuthenticationService/UpdateAuthSessionWithSteamGuardCode/v1", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", contentType)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Eresult") != "1" {
		return fmt.Errorf("invalid X-Eresult header: %s", resp.Header.Get("X-Eresult"))
	}

	return nil
}

func (a *API) PollAuthSessionStatus(
	ctx context.Context,
	req *protocol.CAuthentication_PollAuthSessionStatus_Request,
) (*protocol.CAuthentication_PollAuthSessionStatus_Response, error) {
	if req == nil {
		return nil, errors.New("invalid request")
	}

	bodyBytes, contentType, err := buildProtobufPOSTBody(req)
	if err != nil {
		return nil, fmt.Errorf("build body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.steampowered.com/IAuthenticationService/PollAuthSessionStatus/v1", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", contentType)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Eresult") != "1" {
		return nil, fmt.Errorf("invalid X-Eresult header: %s", resp.Header.Get("X-Eresult"))
	}

	result, err := decodeProtoFromHTTPResponse(resp, &protocol.CAuthentication_PollAuthSessionStatus_Response{})
	if err != nil {
		return nil, err
	}

	debugProto(result)

	return result, nil
}

// buildProtobufPOSTBody builds POST request body compatible with SteamAPI
func buildProtobufPOSTBody(msg proto.Message) (body []byte, contentType string, err error) {
	// TODO:; I think we can return io.Reader instead of []byte
	payload, err := encodeProto(msg)
	if err != nil {
		return nil, "", fmt.Errorf("encode proto: %w", err)
	}

	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	err = w.WriteField("input_protobuf_encoded", payload)
	if err != nil {
		return nil, "", fmt.Errorf("write field: %w", err)
	}

	err = w.Close()
	if err != nil {
		return nil, "", fmt.Errorf("close writer: %w", err)
	}

	return buf.Bytes(), w.FormDataContentType(), nil
}

// encodeProto encodes protobuf messages to base64
func encodeProto(msg proto.Message) (string, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("proto marshal: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// decodeProtoFromHTTPResponse decodes HTTP responses to protobuf messages
func decodeProtoFromHTTPResponse[T proto.Message](resp *http.Response, msg T) (T, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return msg, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return msg, fmt.Errorf("read body: %w", err)
	}

	err = proto.Unmarshal(bodyBytes, msg)
	if err != nil {
		return msg, fmt.Errorf("unmarshal proto: %w", err)
	}

	return msg, nil
}

// debugProto prints all fields and their values in a proto message
func debugProto(msg proto.Message) {
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	fmt.Printf("=== %s ===\n", v.Type().Name())

	t := v.Type()
	for i := range v.NumField() {
		field := t.Field(i)
		value := v.Field(i)

		// Skip unexported/internal fields like sizeCache, unknownFields, etc.
		if !value.CanInterface() {
			continue
		}

		fmt.Printf("  %-20s: ", field.Name)

		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				fmt.Println("<nil>")
				continue
			}
			fmt.Println(value.Elem())
			continue
		}

		if value.Kind() == reflect.Slice {
			if value.IsNil() {
				fmt.Println("<nil>")
			} else {
				fmt.Printf("%v\n", value.Interface())
			}
			continue
		}

		fmt.Printf("%v\n", value.Interface())
	}
	fmt.Println()
}
