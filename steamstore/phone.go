package steamstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// phoneResult is a common response structure for phone-related API calls.
type phoneResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// doPhoneRequest performs a POST request to a phone API endpoint and decodes the response.
func (s *Store) doPhoneRequest(ctx context.Context, endpoint string, formData url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result phoneResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	if !result.Success {
		if result.Error != "" {
			return fmt.Errorf("%s", result.Error)
		}
		return fmt.Errorf("operation failed")
	}

	return nil
}

// PhoneInfo represents information about the user's phone status
type PhoneInfo struct {
	HasPhone        bool
	PhoneEndingWith string // e.g., "79" for "Ends in 79"
}

// HasPhone checks if the account has a phone number attached by parsing the account page
func (s *Store) HasPhone(ctx context.Context) (*PhoneInfo, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://store.steampowered.com/account/",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parsePhoneInfo(string(body))
}

func parsePhoneInfo(html string) (*PhoneInfo, error) {
	info := &PhoneInfo{}

	// Look for "Ends in XX" pattern in phone section
	reEndsIn := regexp.MustCompile(`Phone:\s*<img[^>]*>\s*<span[^>]*>Ends in (\d+)</span>`)
	if m := reEndsIn.FindStringSubmatch(html); len(m) == 2 {
		info.HasPhone = true
		info.PhoneEndingWith = m[1]
		return info, nil
	}

	// Alternative: check if "Manage your phone number" link exists with phone data
	if strings.Contains(html, "phone_header_description") && strings.Contains(html, "Ends in") {
		info.HasPhone = true
		// Try simpler pattern
		reSimple := regexp.MustCompile(`Ends in (\d+)`)
		if m := reSimple.FindStringSubmatch(html); len(m) == 2 {
			info.PhoneEndingWith = m[1]
		}
	}

	return info, nil
}

// AddPhoneNumberRequest contains the data needed to add a phone number
type AddPhoneNumberRequest struct {
	PhoneNumber  string
	PhoneCountry string
}

// AddPhoneNumber adds a phone number to the account.
func (s *Store) AddPhoneNumber(ctx context.Context, req *AddPhoneNumberRequest) error {
	formData := url.Values{
		"sessionid":    {s.sessionID},
		"phoneNumber":  {req.PhoneNumber},
		"phoneCountry": {req.PhoneCountry},
	}
	return s.doPhoneRequest(ctx, "https://store.steampowered.com/phone/add_phone_number", formData)
}

// SendPhoneNumberVerificationMessage sends an SMS verification code to the phone.
func (s *Store) SendPhoneNumberVerificationMessage(ctx context.Context) error {
	formData := url.Values{
		"sessionid": {s.sessionID},
	}
	return s.doPhoneRequest(ctx, "https://store.steampowered.com/phone/send_verification_message", formData)
}

// VerifyPhoneNumber verifies the phone number using the SMS code.
func (s *Store) VerifyPhoneNumber(ctx context.Context, code string) error {
	formData := url.Values{
		"sessionid": {s.sessionID},
		"code":      {code},
	}
	return s.doPhoneRequest(ctx, "https://store.steampowered.com/phone/verify_phone_number", formData)
}

// ResendVerificationSMS resends the verification SMS code.
func (s *Store) ResendVerificationSMS(ctx context.Context) error {
	formData := url.Values{
		"sessionid": {s.sessionID},
	}
	return s.doPhoneRequest(ctx, "https://store.steampowered.com/phone/resend_verification_sms", formData)
}

// RemovePhoneNumber initiates the removal of a phone number from the account.
func (s *Store) RemovePhoneNumber(ctx context.Context) error {
	formData := url.Values{
		"sessionid": {s.sessionID},
	}
	return s.doPhoneRequest(ctx, "https://store.steampowered.com/phone/remove_phone_number", formData)
}

// ConfirmRemovePhoneNumber confirms the removal of a phone number with SMS code.
func (s *Store) ConfirmRemovePhoneNumber(ctx context.Context, code string) error {
	formData := url.Values{
		"sessionid": {s.sessionID},
		"code":      {code},
	}
	return s.doPhoneRequest(ctx, "https://store.steampowered.com/phone/confirm_remove_phone_number", formData)
}
