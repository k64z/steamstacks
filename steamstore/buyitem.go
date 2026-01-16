package steamstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var (
	reTxnID     = regexp.MustCompile(`name="transaction_id"\s+value="(\d+)"`)
	reReturnURL = regexp.MustCompile(`name="returnurl"\s+value="([^"]+)"`)
	reSessionID = regexp.MustCompile(`name="sessionid"\s+value="([^"]+)"`)
	reOrderID   = regexp.MustCompile(`/finalize/(\d+)`)
)

// BuyItemResult represents the result of purchasing an in-game item
type BuyItemResult struct {
	TransactionID string
	OrderID       string
	Success       bool
}

// BuyItem purchases an in-game item from the Steam store.
// appID is the game's app ID (e.g., 440 for TF2).
// itemID is the item definition ID.
func (s *Store) BuyItem(ctx context.Context, appID, itemID int) (*BuyItemResult, error) {
	buyURL := fmt.Sprintf("https://store.steampowered.com/buyitem/%d/%d", appID, itemID)

	txnData, err := s.initBuyItem(ctx, buyURL)
	if err != nil {
		return nil, fmt.Errorf("init buy item: %w", err)
	}

	orderID := extractMatch(reOrderID, txnData.returnURL)

	if err := s.approveTransaction(ctx, txnData); err != nil {
		return nil, fmt.Errorf("approve transaction: %w", err)
	}

	return &BuyItemResult{
		TransactionID: txnData.transactionID,
		OrderID:       orderID,
		Success:       true,
	}, nil
}

type buyItemTxnData struct {
	transactionID string
	returnURL     string
	sessionID     string
}

func (s *Store) initBuyItem(ctx context.Context, buyURL string) (*buyItemTxnData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buyURL, nil)
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

	return parseBuyItemForm(string(body))
}

func parseBuyItemForm(html string) (*buyItemTxnData, error) {
	txnID := extractMatch(reTxnID, html)
	if txnID == "" {
		return nil, fmt.Errorf("transaction_id not found")
	}

	returnURL := extractMatch(reReturnURL, html)
	if returnURL == "" {
		return nil, fmt.Errorf("returnurl not found")
	}

	sessionID := extractMatch(reSessionID, html)
	if sessionID == "" {
		return nil, fmt.Errorf("sessionid not found")
	}

	// Decode HTML entities in returnurl
	returnURL = strings.ReplaceAll(returnURL, "&amp;", "&")

	return &buyItemTxnData{
		transactionID: txnID,
		returnURL:     returnURL,
		sessionID:     sessionID,
	}, nil
}

func extractMatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); len(m) == 2 {
		return m[1]
	}
	return ""
}

func (s *Store) approveTransaction(ctx context.Context, txn *buyItemTxnData) error {
	formData := url.Values{
		"transaction_id": {txn.transactionID},
		"returnurl":      {txn.returnURL},
		"sessionid":      {txn.sessionID},
		"approved":       {"1"},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://checkout.steampowered.com/checkout/approvetxnsubmit",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Success if redirected to inventory after approval
	if strings.Contains(resp.Request.URL.String(), "/inventory") {
		return nil
	}

	// Check for error indicators in response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if strings.Contains(string(body), "error") || strings.Contains(string(body), "failed") {
		return fmt.Errorf("transaction failed")
	}

	return nil
}
