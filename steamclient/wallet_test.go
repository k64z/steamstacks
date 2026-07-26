package steamclient

import (
	"log/slog"
	"testing"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

func walletPacket(t *testing.T, msg *protocol.CMsgClientWalletInfoUpdate) *Packet {
	t.Helper()
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return &Packet{EMsg: EMsgClientWalletInfoUpdate, Body: body}
}

func TestWalletInfoUnsetUntilPushed(t *testing.T) {
	c := &Client{logger: slog.Default()}
	if _, ok := c.WalletInfo(); ok {
		t.Fatal("WalletInfo ok=true before any push; callers would read a zero balance as real")
	}
}

func TestHandleWalletInfoUpdateCachesAndFires(t *testing.T) {
	c := &Client{logger: slog.Default()}
	var fired *WalletInfo
	c.OnWalletInfo = func(w *WalletInfo) { fired = w }

	c.handleWalletInfoUpdate(walletPacket(t, &protocol.CMsgClientWalletInfoUpdate{
		HasWallet:      proto.Bool(true),
		Balance:        proto.Int32(5051),
		BalanceDelayed: proto.Int32(120),
		Currency:       proto.Int32(5),
	}))

	got, ok := c.WalletInfo()
	if !ok {
		t.Fatal("WalletInfo ok=false after a push")
	}
	if got.Balance != 5051 || got.BalanceDelayed != 120 {
		t.Errorf("balance/delayed = %d/%d, want 5051/120", got.Balance, got.BalanceDelayed)
	}
	if got.Currency != 5 || !got.HasWallet {
		t.Errorf("currency/hasWallet = %d/%v, want 5/true", got.Currency, got.HasWallet)
	}
	if got.ReceivedAt.IsZero() {
		t.Error("ReceivedAt not stamped; staleness checks can't work")
	}
	if fired == nil || fired.Balance != 5051 {
		t.Errorf("handler got %+v, want the pushed update", fired)
	}
}

// The 64-bit fields exist for currencies whose balances overflow int32.
// When Steam sends them, they must win over the narrow fields.
func TestHandleWalletInfoUpdatePrefersBalance64(t *testing.T) {
	c := &Client{logger: slog.Default()}
	c.handleWalletInfoUpdate(walletPacket(t, &protocol.CMsgClientWalletInfoUpdate{
		HasWallet:        proto.Bool(true),
		Balance:          proto.Int32(1),
		Balance64:        proto.Int64(9_000_000_000),
		BalanceDelayed:   proto.Int32(2),
		Balance64Delayed: proto.Int64(8_000_000_000),
		Currency:         proto.Int32(37),
	}))

	got, _ := c.WalletInfo()
	if got.Balance != 9_000_000_000 {
		t.Errorf("Balance = %d, want the 64-bit value", got.Balance)
	}
	if got.BalanceDelayed != 8_000_000_000 {
		t.Errorf("BalanceDelayed = %d, want the 64-bit value", got.BalanceDelayed)
	}
}

// A later push must replace the cache, not merge into it — otherwise a
// spent-down wallet keeps reporting its old balance.
func TestHandleWalletInfoUpdateReplacesCache(t *testing.T) {
	c := &Client{logger: slog.Default()}
	c.handleWalletInfoUpdate(walletPacket(t, &protocol.CMsgClientWalletInfoUpdate{
		HasWallet: proto.Bool(true), Balance: proto.Int32(5051), Currency: proto.Int32(5),
	}))
	c.handleWalletInfoUpdate(walletPacket(t, &protocol.CMsgClientWalletInfoUpdate{
		HasWallet: proto.Bool(true), Balance: proto.Int32(11), Currency: proto.Int32(5),
	}))

	got, _ := c.WalletInfo()
	if got.Balance != 11 {
		t.Errorf("Balance = %d, want 11 from the newer push", got.Balance)
	}
}
