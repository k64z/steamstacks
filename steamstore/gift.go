package steamstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/k64z/steamstacks/steamid"
)

// SendGiftRequest contains the data needed to send a gift
type SendGiftRequest struct {
	GiftID       string          // The gift ID to send
	RecipientID  steamid.SteamID // The recipient's SteamID
	GiftMessage  string          // Optional message to include with the gift
	GiftSentence string          // Optional sentence/greeting
	Signature    string          // Optional signature
}

// SendGift sends a gift to another Steam user
func (s *Store) SendGift(ctx context.Context, req *SendGiftRequest) error {
	formData := url.Values{
		"sessionid":      {s.sessionID},
		"giftid":         {req.GiftID},
		"steamid_friend": {fmt.Sprintf("%d", req.RecipientID.ToSteamID64())},
		"gift_message":   {req.GiftMessage},
		"gift_sentence":  {req.GiftSentence},
		"gift_signature": {req.Signature},
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://store.steampowered.com/gifts/sendgift",
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
		Success EResult `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	if result.Success != EResultOK {
		return NewStoreError(result.Success, "")
	}

	return nil
}
