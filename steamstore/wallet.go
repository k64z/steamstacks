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

// WalletBalance represents the user's Steam Wallet balance
type WalletBalance struct {
	Balance     string // Formatted balance (e.g., "$10.00")
	HasWallet   bool
	Currency    string
}

// RedeemWalletCodeResult represents the result of redeeming a wallet code
type RedeemWalletCodeResult struct {
	Success        EResult         `json:"success"`
	PurchaseResult EPurchaseResult `json:"purchase_result_details"`
	Amount         string          `json:"formattedNewWalletBalance"`
	Detail         int             `json:"detail"`
}

// GetWalletBalance retrieves the user's Steam Wallet balance by scraping the account page
func (s *Store) GetWalletBalance(ctx context.Context) (*WalletBalance, error) {
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

	return parseWalletBalance(string(body))
}

func parseWalletBalance(html string) (*WalletBalance, error) {
	// Look for wallet balance in the account page HTML
	// Pattern: <a class="global_action_link" id="header_wallet_balance">$X.XX</a>
	reBalance := regexp.MustCompile(`<a[^>]*id="header_wallet_balance"[^>]*>([^<]+)</a>`)

	balance := &WalletBalance{}

	if m := reBalance.FindStringSubmatch(html); len(m) == 2 {
		balance.Balance = strings.TrimSpace(m[1])
		balance.HasWallet = true
	}

	return balance, nil
}

// RedeemWalletCode redeems a Steam Wallet code
func (s *Store) RedeemWalletCode(ctx context.Context, code string) (*RedeemWalletCodeResult, error) {
	formData := url.Values{
		"wallet_code": {code},
		"sessionid":   {s.sessionID},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://store.steampowered.com/account/ajaxredeemwalletcode/",
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

	var result RedeemWalletCodeResult
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

// CreateWalletRequest contains the billing information for creating a wallet
type CreateWalletRequest struct {
	WalletCode     string
	CreateFromCode int
	BillingAddress struct {
		FirstName   string
		LastName    string
		Address     string
		City        string
		Country     string
		State       string
		PostalCode  string
		Phone       string
	}
}

// CreateWallet creates a new Steam Wallet using a wallet code and billing information
func (s *Store) CreateWallet(ctx context.Context, req *CreateWalletRequest) error {
	formData := url.Values{
		"wallet_code":     {req.WalletCode},
		"CreateFromCode":  {fmt.Sprintf("%d", req.CreateFromCode)},
		"sessionid":       {s.sessionID},
		"billing_first":   {req.BillingAddress.FirstName},
		"billing_last":    {req.BillingAddress.LastName},
		"billing_address": {req.BillingAddress.Address},
		"billing_city":    {req.BillingAddress.City},
		"billing_country": {req.BillingAddress.Country},
		"billing_state":   {req.BillingAddress.State},
		"billing_postal":  {req.BillingAddress.PostalCode},
		"billing_phone":   {req.BillingAddress.Phone},
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://store.steampowered.com/account/createwallet/",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Success        EResult         `json:"success"`
		PurchaseResult EPurchaseResult `json:"purchase_result_details"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	if result.Success != EResultOK {
		return NewStoreError(result.Success, "")
	}

	if result.PurchaseResult != EPurchaseResultOK {
		return NewPurchaseError(result.PurchaseResult, "")
	}

	return nil
}
