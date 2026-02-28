package tf2

import (
	"math"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestDecodeItem(t *testing.T) {
	var b []byte
	// field 1: id = 9876543210
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, 9876543210)
	// field 2: account_id = 12345
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, 12345)
	// field 3: inventory = 2147483907 (position=259, new bit not set)
	b = protowire.AppendTag(b, 3, protowire.VarintType)
	b = protowire.AppendVarint(b, 2147483907)
	// field 4: def_index = 5021
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, 5021)
	// field 5: quantity = 1
	b = protowire.AppendTag(b, 5, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	// field 6: level = 5
	b = protowire.AppendTag(b, 6, protowire.VarintType)
	b = protowire.AppendVarint(b, 5)
	// field 7: quality = 6
	b = protowire.AppendTag(b, 7, protowire.VarintType)
	b = protowire.AppendVarint(b, 6)
	// field 8: flags = 0
	b = protowire.AppendTag(b, 8, protowire.VarintType)
	b = protowire.AppendVarint(b, 0)
	// field 9: origin = 4
	b = protowire.AppendTag(b, 9, protowire.VarintType)
	b = protowire.AppendVarint(b, 4)
	// field 10: custom_name = "My Item"
	b = protowire.AppendTag(b, 10, protowire.BytesType)
	b = protowire.AppendString(b, "My Item")
	// field 11: custom_desc = "Cool description"
	b = protowire.AppendTag(b, 11, protowire.BytesType)
	b = protowire.AppendString(b, "Cool description")
	// field 14: in_use = true
	b = protowire.AppendTag(b, 14, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	// field 15: style = 2
	b = protowire.AppendTag(b, 15, protowire.VarintType)
	b = protowire.AppendVarint(b, 2)
	// field 16: original_id = 9876543200
	b = protowire.AppendTag(b, 16, protowire.VarintType)
	b = protowire.AppendVarint(b, 9876543200)

	item, err := decodeItem(b)
	if err != nil {
		t.Fatalf("decodeItem: %v", err)
	}

	if item.ID != 9876543210 {
		t.Errorf("ID = %d, want 9876543210", item.ID)
	}
	if item.AccountID != 12345 {
		t.Errorf("AccountID = %d, want 12345", item.AccountID)
	}
	if item.Inventory != 2147483907 {
		t.Errorf("Inventory = %d, want 2147483907", item.Inventory)
	}
	if item.DefIndex != 5021 {
		t.Errorf("DefIndex = %d, want 5021", item.DefIndex)
	}
	if item.Quantity != 1 {
		t.Errorf("Quantity = %d, want 1", item.Quantity)
	}
	if item.Level != 5 {
		t.Errorf("Level = %d, want 5", item.Level)
	}
	if item.Quality != 6 {
		t.Errorf("Quality = %d, want 6", item.Quality)
	}
	if item.Flags != 0 {
		t.Errorf("Flags = %d, want 0", item.Flags)
	}
	if item.Origin != 4 {
		t.Errorf("Origin = %d, want 4", item.Origin)
	}
	if item.CustomName != "My Item" {
		t.Errorf("CustomName = %q, want %q", item.CustomName, "My Item")
	}
	if item.CustomDesc != "Cool description" {
		t.Errorf("CustomDesc = %q, want %q", item.CustomDesc, "Cool description")
	}
	if !item.InUse {
		t.Error("InUse = false, want true")
	}
	if item.Style != 2 {
		t.Errorf("Style = %d, want 2", item.Style)
	}
	if item.OriginalID != 9876543200 {
		t.Errorf("OriginalID = %d, want 9876543200", item.OriginalID)
	}
	// Position derived from Inventory: 2147483907 & 0xFFFF = 259, new bit not set.
	if item.Position != 259 {
		t.Errorf("Position = %d, want 259", item.Position)
	}
	if item.IsNew {
		t.Error("IsNew = true, want false")
	}
}

func TestDecodeItemNewBit(t *testing.T) {
	// Inventory with bit 30 set: 0x40000103 = 1073741059
	inv := uint32(1<<30 | 0x103)
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, 100)
	b = protowire.AppendTag(b, 3, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(inv))

	item, err := decodeItem(b)
	if err != nil {
		t.Fatalf("decodeItem: %v", err)
	}

	if !item.IsNew {
		t.Error("IsNew = false, want true")
	}
	if item.Position != 0 {
		t.Errorf("Position = %d, want 0 when IsNew", item.Position)
	}
}

func TestDecodeItemAttributes(t *testing.T) {
	// Build an attribute submessage: def_index=134, value=float32 bits for 1.5
	var attr1 []byte
	attr1 = protowire.AppendTag(attr1, 1, protowire.VarintType)
	attr1 = protowire.AppendVarint(attr1, 134)
	attr1 = protowire.AppendTag(attr1, 2, protowire.VarintType)
	attr1 = protowire.AppendVarint(attr1, uint64(math.Float32bits(1.5)))

	// Build an attribute with value_bytes.
	var attr2 []byte
	attr2 = protowire.AppendTag(attr2, 1, protowire.VarintType)
	attr2 = protowire.AppendVarint(attr2, 200)
	attr2 = protowire.AppendTag(attr2, 3, protowire.BytesType)
	attr2 = protowire.AppendBytes(attr2, []byte{0xDE, 0xAD})

	// Build equipped state: class=3, slot=0
	var eq []byte
	eq = protowire.AppendTag(eq, 1, protowire.VarintType)
	eq = protowire.AppendVarint(eq, 3)
	eq = protowire.AppendTag(eq, 2, protowire.VarintType)
	eq = protowire.AppendVarint(eq, 0)

	// Build item with attributes and equipped state.
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, 42)
	b = protowire.AppendTag(b, 12, protowire.BytesType)
	b = protowire.AppendBytes(b, attr1)
	b = protowire.AppendTag(b, 12, protowire.BytesType)
	b = protowire.AppendBytes(b, attr2)
	b = protowire.AppendTag(b, 18, protowire.BytesType)
	b = protowire.AppendBytes(b, eq)

	item, err := decodeItem(b)
	if err != nil {
		t.Fatalf("decodeItem: %v", err)
	}

	if len(item.Attributes) != 2 {
		t.Fatalf("got %d attributes, want 2", len(item.Attributes))
	}
	if item.Attributes[0].DefIndex != 134 {
		t.Errorf("attr[0].DefIndex = %d, want 134", item.Attributes[0].DefIndex)
	}
	if item.Attributes[0].Value != math.Float32bits(1.5) {
		t.Errorf("attr[0].Value = %d, want %d", item.Attributes[0].Value, math.Float32bits(1.5))
	}
	if item.Attributes[1].DefIndex != 200 {
		t.Errorf("attr[1].DefIndex = %d, want 200", item.Attributes[1].DefIndex)
	}
	if len(item.Attributes[1].ValueBytes) != 2 || item.Attributes[1].ValueBytes[0] != 0xDE {
		t.Errorf("attr[1].ValueBytes = %x, want DEAD", item.Attributes[1].ValueBytes)
	}

	if len(item.EquipState) != 1 {
		t.Fatalf("got %d equip states, want 1", len(item.EquipState))
	}
	if item.EquipState[0].NewClass != 3 {
		t.Errorf("equip[0].NewClass = %d, want 3", item.EquipState[0].NewClass)
	}
	if item.EquipState[0].NewSlot != 0 {
		t.Errorf("equip[0].NewSlot = %d, want 0", item.EquipState[0].NewSlot)
	}
}

func TestDecodeItemUnknownFields(t *testing.T) {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, 55)
	// field 13: interior_item (bytes, unknown submessage — should be skipped)
	b = protowire.AppendTag(b, 13, protowire.BytesType)
	b = protowire.AppendBytes(b, []byte{0x08, 0x01})
	// field 17: contains_equipped_state (varint — should be skipped)
	b = protowire.AppendTag(b, 17, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	// field 19: contains_equipped_state_v2 (varint — should be skipped)
	b = protowire.AppendTag(b, 19, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	// field 99: completely unknown field
	b = protowire.AppendTag(b, 99, protowire.VarintType)
	b = protowire.AppendVarint(b, 999)

	item, err := decodeItem(b)
	if err != nil {
		t.Fatalf("decodeItem with unknown fields: %v", err)
	}
	if item.ID != 55 {
		t.Errorf("ID = %d, want 55", item.ID)
	}
}

func TestDecodeAccount(t *testing.T) {
	var b []byte
	// field 1: additional_backpack_slots = 100
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, 100)
	// field 2: trial_account = false
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, 0)
	// field 4: need_to_choose_most_helpful_friend = true
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	// field 5: in_coaches_list = true
	b = protowire.AppendTag(b, 5, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	// field 6: trade_ban_expiration = 1700000000
	b = protowire.AppendTag(b, 6, protowire.Fixed32Type)
	b = protowire.AppendFixed32(b, 1700000000)
	// field 7: duel_ban_expiration = 0
	b = protowire.AppendTag(b, 7, protowire.Fixed32Type)
	b = protowire.AppendFixed32(b, 0)
	// field 19: phone_verified = true
	b = protowire.AppendTag(b, 19, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	// field 23: competitive_access = true
	b = protowire.AppendTag(b, 23, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)

	acc, err := decodeAccount(b)
	if err != nil {
		t.Fatalf("decodeAccount: %v", err)
	}

	if acc.AdditionalBackpackSlots != 100 {
		t.Errorf("AdditionalBackpackSlots = %d, want 100", acc.AdditionalBackpackSlots)
	}
	if acc.TrialAccount {
		t.Error("TrialAccount = true, want false")
	}
	if !acc.Premium {
		t.Error("Premium = false, want true")
	}
	if acc.BackpackSlots != 400 {
		t.Errorf("BackpackSlots = %d, want 400 (300+100)", acc.BackpackSlots)
	}
	if !acc.NeedToChooseMostHelpfulFriend {
		t.Error("NeedToChooseMostHelpfulFriend = false, want true")
	}
	if !acc.InCoachesList {
		t.Error("InCoachesList = false, want true")
	}
	if acc.TradeBanExpiration != 1700000000 {
		t.Errorf("TradeBanExpiration = %d, want 1700000000", acc.TradeBanExpiration)
	}
	if acc.DuelBanExpiration != 0 {
		t.Errorf("DuelBanExpiration = %d, want 0", acc.DuelBanExpiration)
	}
	if !acc.PhoneVerified {
		t.Error("PhoneVerified = false, want true")
	}
	if !acc.CompetitiveAccess {
		t.Error("CompetitiveAccess = false, want true")
	}
}

func TestDecodeAccountTrial(t *testing.T) {
	var b []byte
	// field 1: additional_backpack_slots = 10
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, 10)
	// field 2: trial_account = true
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)

	acc, err := decodeAccount(b)
	if err != nil {
		t.Fatalf("decodeAccount: %v", err)
	}

	if !acc.TrialAccount {
		t.Error("TrialAccount = false, want true")
	}
	if acc.Premium {
		t.Error("Premium = true, want false")
	}
	if acc.BackpackSlots != 60 {
		t.Errorf("BackpackSlots = %d, want 60 (50+10)", acc.BackpackSlots)
	}
}
