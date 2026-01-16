package steamstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SetDisplayLanguages sets the user's preferred display languages
// primaryLang is required, secondaryLang is optional (can be empty string)
func (s *Store) SetDisplayLanguages(ctx context.Context, primaryLang, secondaryLang string) error {
	formData := url.Values{
		"sessionid":      {s.sessionID},
		"primary_lang":   {primaryLang},
		"secondary_lang": {secondaryLang},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://store.steampowered.com/account/setlanguagepreferences",
		strings.NewReader(formData.Encode()),
	)
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

	var result struct {
		Success int `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	if result.Success != 1 {
		return fmt.Errorf("unexpected success value: %d", result.Success)
	}

	return nil
}
