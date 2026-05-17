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

const phoneAjaxURL = "https://store.steampowered.com/phone/add_ajaxop"

// PhoneAjaxResponse is the response from the phone add_ajaxop endpoint.
// Note: Steam returns inconsistent types (e.g. state is string on success, false on error).
type PhoneAjaxResponse struct {
	Success              bool   `json:"success"`
	State                string `json:"-"` // parsed manually; Steam sends false (bool) on error
	ErrorText            string `json:"errorText"`
	RequiresConfirmation bool   `json:"requiresConfirmation"`
	ConfirmationText     string `json:"confirmationText"`
	PhoneNumber          string `json:"phoneNumber"`
	DefaultText          string `json:"defaultText"`
	InputSize            string `json:"inputSize"`
	MaxLength            string `json:"maxLength"`
	PhoneTOSViolation    bool   `json:"phone_tos_violation"`
}

type phoneAjaxRaw struct {
	PhoneAjaxResponse
	RawState json.RawMessage `json:"state"`
}

// phoneAjaxOp sends a POST to add_ajaxop with the given op and input.
func (s *Store) phoneAjaxOp(ctx context.Context, op, input string, confirmed bool) (*PhoneAjaxResponse, error) {
	confirmedStr := "0"
	if confirmed {
		confirmedStr = "1"
	}

	formData := url.Values{
		"op":          {op},
		"input":       {input},
		"sessionID":   {s.sessionID},
		"confirmed":   {confirmedStr},
		"checkfortos": {"1"},
		"bisediting":  {"0"},
		"token":       {"0"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, phoneAjaxURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

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

	var raw phoneAjaxRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500]
		}
		return nil, fmt.Errorf("decode JSON: %w (response: %s)", err, preview)
	}

	result := &raw.PhoneAjaxResponse
	// State can be a string or false (bool) on error
	if len(raw.RawState) > 0 && raw.RawState[0] == '"' {
		json.Unmarshal(raw.RawState, &result.State)
	}

	if result.ErrorText != "" {
		return result, fmt.Errorf("%s", result.ErrorText)
	}

	if result.PhoneTOSViolation {
		return result, fmt.Errorf("phone TOS violation")
	}

	return result, nil
}

// AddPhoneNumber submits a phone number (op=get_phone_number).
func (s *Store) AddPhoneNumber(ctx context.Context, phoneNumber string) (*PhoneAjaxResponse, error) {
	return s.phoneAjaxOp(ctx, "get_phone_number", phoneNumber, false)
}

// AddPhoneNumberConfirmed resubmits with confirmed=true (for reused numbers).
func (s *Store) AddPhoneNumberConfirmed(ctx context.Context, phoneNumber string) (*PhoneAjaxResponse, error) {
	return s.phoneAjaxOp(ctx, "get_phone_number", phoneNumber, true)
}

// ConfirmEmailVerification signals that the email link was clicked (op=email_verification).
func (s *Store) ConfirmEmailVerification(ctx context.Context) (*PhoneAjaxResponse, error) {
	return s.phoneAjaxOp(ctx, "email_verification", "", false)
}

// RetryEmailVerification retries email verification (op=retry_email_verification).
func (s *Store) RetryEmailVerification(ctx context.Context) (*PhoneAjaxResponse, error) {
	return s.phoneAjaxOp(ctx, "retry_email_verification", "", false)
}

// SubmitSMSCode submits the SMS verification code (op=get_sms_code).
func (s *Store) SubmitSMSCode(ctx context.Context, code string) (*PhoneAjaxResponse, error) {
	return s.phoneAjaxOp(ctx, "get_sms_code", code, false)
}

// ResendSMS resends the SMS code (op=resend_sms).
func (s *Store) ResendSMS(ctx context.Context) (*PhoneAjaxResponse, error) {
	return s.phoneAjaxOp(ctx, "resend_sms", "", false)
}

// PhoneInfo represents information about the user's phone status.
type PhoneInfo struct {
	HasPhone        bool
	PhoneEndingWith string
}

// HasPhone checks if the account has a phone number attached by parsing the account page.
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

	reEndsIn := regexp.MustCompile(`Phone:\s*<img[^>]*>\s*<span[^>]*>Ends in (\d+)</span>`)
	if m := reEndsIn.FindStringSubmatch(html); len(m) == 2 {
		info.HasPhone = true
		info.PhoneEndingWith = m[1]
		return info, nil
	}

	if strings.Contains(html, "phone_header_description") && strings.Contains(html, "Ends in") {
		info.HasPhone = true
		reSimple := regexp.MustCompile(`Ends in (\d+)`)
		if m := reSimple.FindStringSubmatch(html); len(m) == 2 {
			info.PhoneEndingWith = m[1]
		}
	}

	return info, nil
}
