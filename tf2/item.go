package tf2

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// Item represents a CSOEconItem from the TF2 backpack.
type Item struct {
	ID         uint64
	AccountID  uint32
	Inventory  uint32
	DefIndex   uint32
	Quantity   uint32
	Level      uint32
	Quality    uint32
	Flags      uint32
	Origin     uint32
	CustomName string
	CustomDesc string
	Style      uint32
	OriginalID uint64
	InUse      bool
	Position   uint16 // derived: Inventory & 0xFFFF (0 if new bit set)
	IsNew      bool   // derived: bit 30 of Inventory
	Attributes []ItemAttribute
	EquipState []ItemEquipped
}

// ItemAttribute represents a CSOEconItemAttribute.
type ItemAttribute struct {
	DefIndex   uint32
	Value      uint32 // float bits stored as uint32
	ValueBytes []byte
}

// ItemEquipped represents a CSOEconItemEquipped.
type ItemEquipped struct {
	NewClass uint32
	NewSlot  uint32
}

// Account represents a CSOEconGameAccountClient.
type Account struct {
	AdditionalBackpackSlots       uint32
	TrialAccount                  bool
	Premium                       bool   // derived: !TrialAccount
	BackpackSlots                 uint32 // derived: (trial?50:300) + AdditionalBackpackSlots
	NeedToChooseMostHelpfulFriend bool
	InCoachesList                 bool
	TradeBanExpiration            uint32 // fixed32
	DuelBanExpiration             uint32 // fixed32
	PhoneVerified                 bool
	CompetitiveAccess             bool
}

func decodeItem(b []byte) (Item, error) {
	var it Item
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return it, fmt.Errorf("decodeItem: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return it, fmt.Errorf("decodeItem: invalid varint field %d", num)
			}
			b = b[n:]
			switch num {
			case 1:
				it.ID = v
			case 2:
				it.AccountID = uint32(v)
			case 3:
				it.Inventory = uint32(v)
			case 4:
				it.DefIndex = uint32(v)
			case 5:
				it.Quantity = uint32(v)
			case 6:
				it.Level = uint32(v)
			case 7:
				it.Quality = uint32(v)
			case 8:
				it.Flags = uint32(v)
			case 9:
				it.Origin = uint32(v)
			case 14:
				it.InUse = v != 0
			case 15:
				it.Style = uint32(v)
			case 16:
				it.OriginalID = v
			}

		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return it, fmt.Errorf("decodeItem: invalid bytes field %d", num)
			}
			b = b[n:]
			switch num {
			case 10:
				it.CustomName = string(v)
			case 11:
				it.CustomDesc = string(v)
			case 12:
				attr, err := decodeItemAttribute(v)
				if err != nil {
					return it, fmt.Errorf("decodeItem: attribute: %w", err)
				}
				it.Attributes = append(it.Attributes, attr)
			case 18:
				eq, err := decodeItemEquipped(v)
				if err != nil {
					return it, fmt.Errorf("decodeItem: equipped_state: %w", err)
				}
				it.EquipState = append(it.EquipState, eq)
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return it, fmt.Errorf("decodeItem: cannot skip field %d wire %d", num, wtype)
			}
			b = b[n:]
		}
	}

	// Derive Position and IsNew from Inventory.
	it.IsNew = (it.Inventory>>30)&1 == 1
	if it.IsNew {
		it.Position = 0
	} else {
		it.Position = uint16(it.Inventory & 0xFFFF)
	}

	return it, nil
}

func decodeItemAttribute(b []byte) (ItemAttribute, error) {
	var a ItemAttribute
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return a, fmt.Errorf("decodeItemAttribute: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return a, fmt.Errorf("decodeItemAttribute: invalid varint field %d", num)
			}
			b = b[n:]
			switch num {
			case 1:
				a.DefIndex = uint32(v)
			case 2:
				a.Value = uint32(v)
			}

		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return a, fmt.Errorf("decodeItemAttribute: invalid bytes field %d", num)
			}
			b = b[n:]
			if num == 3 {
				cp := make([]byte, len(v))
				copy(cp, v)
				a.ValueBytes = cp
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return a, fmt.Errorf("decodeItemAttribute: cannot skip field %d", num)
			}
			b = b[n:]
		}
	}
	return a, nil
}

func decodeItemEquipped(b []byte) (ItemEquipped, error) {
	var e ItemEquipped
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return e, fmt.Errorf("decodeItemEquipped: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return e, fmt.Errorf("decodeItemEquipped: invalid varint field %d", num)
			}
			b = b[n:]
			switch num {
			case 1:
				e.NewClass = uint32(v)
			case 2:
				e.NewSlot = uint32(v)
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return e, fmt.Errorf("decodeItemEquipped: cannot skip field %d", num)
			}
			b = b[n:]
		}
	}
	return e, nil
}

func decodeAccount(b []byte) (Account, error) {
	var a Account
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return a, fmt.Errorf("decodeAccount: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return a, fmt.Errorf("decodeAccount: invalid varint field %d", num)
			}
			b = b[n:]
			switch num {
			case 1:
				a.AdditionalBackpackSlots = uint32(v)
			case 2:
				a.TrialAccount = v != 0
			case 4:
				a.NeedToChooseMostHelpfulFriend = v != 0
			case 5:
				a.InCoachesList = v != 0
			case 19:
				a.PhoneVerified = v != 0
			case 23:
				a.CompetitiveAccess = v != 0
			}

		case protowire.Fixed32Type:
			v, n := protowire.ConsumeFixed32(b)
			if n < 0 {
				return a, fmt.Errorf("decodeAccount: invalid fixed32 field %d", num)
			}
			b = b[n:]
			switch num {
			case 6:
				a.TradeBanExpiration = v
			case 7:
				a.DuelBanExpiration = v
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return a, fmt.Errorf("decodeAccount: cannot skip field %d", num)
			}
			b = b[n:]
		}
	}

	// Derive Premium and BackpackSlots.
	a.Premium = !a.TrialAccount
	base := uint32(300)
	if a.TrialAccount {
		base = 50
	}
	a.BackpackSlots = base + a.AdditionalBackpackSlots

	return a, nil
}

