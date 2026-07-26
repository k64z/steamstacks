package steamclient

import (
	"time"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

// WalletInfo is the account's wallet state as pushed by the CM. Steam sends
// EMsgClientWalletInfoUpdate unsolicited shortly after logon and again on
// every balance change, so a logged-in client holds a current balance without
// ever issuing an HTTP request.
//
// Prefer this over the web reads (steamcommunity GetWalletBalance / the
// g_rgWalletInfo scrape) wherever a CM session exists: it is free, it is
// pushed rather than polled, and it cannot be rate-limited. It carries no
// market *fee* configuration — that lives only in g_rgWalletInfo on the
// /market/ home page.
type WalletInfo struct {
	// HasWallet is false for accounts with no wallet in their region.
	HasWallet bool
	// Balance is spendable funds in minor units of Currency. Balance64 is
	// the same figure in the wider field Steam added for high-inflation
	// currencies; Balance is populated from it when it fits, so callers can
	// keep using Balance.
	Balance int64
	// BalanceDelayed is funds received but not yet clear to spend (market
	// proceeds inside Steam's holding window).
	BalanceDelayed int64
	// Currency is Steam's numeric currency id (ECurrencyCode: 1=USD, 3=EUR,
	// 5=RUB, 37=KZT …) — the same id space g_rgWalletInfo's wallet_currency
	// uses, so no ISO-code mapping is needed.
	Currency int32
	// ReceivedAt is when this client received the push, for staleness
	// checks by callers that fall back to a web read.
	ReceivedAt time.Time
}

// WithWalletInfoHandler sets a callback fired on every wallet update the CM
// pushes. Optional — the value is cached regardless and readable via
// Client.WalletInfo.
func WithWalletInfoHandler(fn func(*WalletInfo)) Option {
	return func(c *config) { c.onWalletInfo = fn }
}

// WalletInfo returns the most recent wallet state pushed by the CM, and
// whether one has arrived at all. False means "nothing pushed yet" — a
// freshly connected client, or an account Steam never sent an update for —
// and callers should fall back to a web read rather than treat it as a zero
// balance.
func (c *Client) WalletInfo() (WalletInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.walletInfo == nil {
		return WalletInfo{}, false
	}
	return *c.walletInfo, true
}

// handleWalletInfoUpdate processes an EMsgClientWalletInfoUpdate packet.
func (c *Client) handleWalletInfoUpdate(pkt *Packet) {
	var msg protocol.CMsgClientWalletInfoUpdate
	if err := proto.Unmarshal(pkt.Body, &msg); err != nil {
		c.logger.Error("unmarshal WalletInfoUpdate", "err", err)
		return
	}

	// Steam populates the 32-bit fields for most currencies and the 64-bit
	// ones for the rest; take whichever is set so a KZT-sized balance isn't
	// silently truncated.
	info := WalletInfo{
		HasWallet:      msg.GetHasWallet(),
		Balance:        int64(msg.GetBalance()),
		BalanceDelayed: int64(msg.GetBalanceDelayed()),
		Currency:       msg.GetCurrency(),
		ReceivedAt:     time.Now(),
	}
	if b := msg.GetBalance64(); b != 0 {
		info.Balance = b
	}
	if b := msg.GetBalance64Delayed(); b != 0 {
		info.BalanceDelayed = b
	}

	c.mu.Lock()
	c.walletInfo = &info
	c.mu.Unlock()

	if c.OnWalletInfo != nil {
		c.OnWalletInfo(&info)
	}
}
