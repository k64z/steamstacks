package steamstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// AddFreeLicenseResult represents the result of adding a free license
type AddFreeLicenseResult struct {
	Success        EResult         `json:"success"`
	PurchaseResult EPurchaseResult `json:"purchaseresultdetail"`
}

// AddFreeLicense adds a free license (subscription) to the account
func (s *Store) AddFreeLicense(ctx context.Context, subID int) (*AddFreeLicenseResult, error) {
	formData := url.Values{
		"sessionid": {s.sessionID},
		"subid":     {fmt.Sprintf("%d", subID)},
		"action":    {"add_to_cart"},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://store.steampowered.com/checkout/addfreelicense",
		strings.NewReader(formData.Encode()),
	)
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

	var result AddFreeLicenseResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	if result.Success != EResultOK {
		return &result, NewStoreError(result.Success, "")
	}

	if result.PurchaseResult != EPurchaseResultOK {
		return &result, NewPurchaseError(result.PurchaseResult, "")
	}

	return &result, nil
}

// RemoveLicense removes a license (subscription) from the account
func (s *Store) RemoveLicense(ctx context.Context, subID int) error {
	formData := url.Values{
		"sessionid": {s.sessionID},
		"packageid": {fmt.Sprintf("%d", subID)},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://store.steampowered.com/account/removelicense",
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
