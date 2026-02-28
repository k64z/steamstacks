package tf2

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// SO type IDs used by the TF2 Game Coordinator.
const (
	SOTypeItem    = 1
	SOTypeAccount = 7
)

// SO message types from the GC SDK.
const (
	MsgSOCreate                    = 21
	MsgSOUpdate                    = 22
	MsgSODestroy                   = 23
	MsgSOCacheSubscribed           = 24
	MsgSOUpdateMultiple            = 26
	MsgSOCacheSubscriptionCheck    = 27
	MsgSOCacheSubscriptionRefresh  = 28
)

// soCache stores the live SO cache state: backpack items and account metadata.
type soCache struct {
	items   map[uint64]*Item
	account *Account
}

func newSOCache() *soCache {
	return &soCache{items: make(map[uint64]*Item)}
}

func (c *soCache) reset() {
	c.items = make(map[uint64]*Item)
	c.account = nil
}

// --- SO message decoders (protowire) ---

type subscribedType struct {
	typeID     int32
	objectData [][]byte
}

type singleObject struct {
	typeID     int32
	objectData []byte
}

func decodeCacheSubscribed(b []byte) (uint64, []subscribedType, error) {
	var owner uint64
	var objects []subscribedType
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return 0, nil, fmt.Errorf("decodeCacheSubscribed: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.Fixed64Type:
			v, n := protowire.ConsumeFixed64(b)
			if n < 0 {
				return 0, nil, fmt.Errorf("decodeCacheSubscribed: invalid fixed64 field %d", num)
			}
			b = b[n:]
			if num == 1 {
				owner = v
			}

		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return 0, nil, fmt.Errorf("decodeCacheSubscribed: invalid bytes field %d", num)
			}
			b = b[n:]
			if num == 2 {
				st, err := decodeSubscribedType(v)
				if err != nil {
					return 0, nil, fmt.Errorf("decodeCacheSubscribed: %w", err)
				}
				objects = append(objects, st)
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return 0, nil, fmt.Errorf("decodeCacheSubscribed: cannot skip field %d", num)
			}
			b = b[n:]
		}
	}
	return owner, objects, nil
}

func decodeSubscribedType(b []byte) (subscribedType, error) {
	var st subscribedType
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return st, fmt.Errorf("decodeSubscribedType: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return st, fmt.Errorf("decodeSubscribedType: invalid varint field %d", num)
			}
			b = b[n:]
			if num == 1 {
				st.typeID = int32(v)
			}

		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return st, fmt.Errorf("decodeSubscribedType: invalid bytes field %d", num)
			}
			b = b[n:]
			if num == 2 {
				cp := make([]byte, len(v))
				copy(cp, v)
				st.objectData = append(st.objectData, cp)
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return st, fmt.Errorf("decodeSubscribedType: cannot skip field %d", num)
			}
			b = b[n:]
		}
	}
	return st, nil
}

func decodeSingleObject(b []byte) (singleObject, error) {
	var so singleObject
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return so, fmt.Errorf("decodeSingleObject: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return so, fmt.Errorf("decodeSingleObject: invalid varint field %d", num)
			}
			b = b[n:]
			if num == 2 {
				so.typeID = int32(v)
			}

		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return so, fmt.Errorf("decodeSingleObject: invalid bytes field %d", num)
			}
			b = b[n:]
			if num == 3 {
				cp := make([]byte, len(v))
				copy(cp, v)
				so.objectData = cp
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return so, fmt.Errorf("decodeSingleObject: cannot skip field %d", num)
			}
			b = b[n:]
		}
	}
	return so, nil
}

func decodeMultipleObjects(b []byte) ([]singleObject, error) {
	var objects []singleObject
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, fmt.Errorf("decodeMultipleObjects: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, fmt.Errorf("decodeMultipleObjects: invalid bytes field %d", num)
			}
			b = b[n:]
			if num == 2 {
				so, err := decodeMultipleObjectsEntry(v)
				if err != nil {
					return nil, fmt.Errorf("decodeMultipleObjects: %w", err)
				}
				objects = append(objects, so)
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return nil, fmt.Errorf("decodeMultipleObjects: cannot skip field %d", num)
			}
			b = b[n:]
		}
	}
	return objects, nil
}

func decodeMultipleObjectsEntry(b []byte) (singleObject, error) {
	var so singleObject
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return so, fmt.Errorf("decodeMultipleObjectsEntry: invalid tag")
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return so, fmt.Errorf("decodeMultipleObjectsEntry: invalid varint field %d", num)
			}
			b = b[n:]
			if num == 1 {
				so.typeID = int32(v)
			}

		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return so, fmt.Errorf("decodeMultipleObjectsEntry: invalid bytes field %d", num)
			}
			b = b[n:]
			if num == 2 {
				cp := make([]byte, len(v))
				copy(cp, v)
				so.objectData = cp
			}

		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return so, fmt.Errorf("decodeMultipleObjectsEntry: cannot skip field %d", num)
			}
			b = b[n:]
		}
	}
	return so, nil
}

// --- Handler methods on *Client ---

func (c *Client) handleSOCacheSubscribed(body []byte) {
	_, objects, err := decodeCacheSubscribed(body)
	if err != nil {
		c.logger.Error("tf2: decode SO cache subscribed", "err", err)
		return
	}

	for _, obj := range objects {
		switch obj.typeID {
		case SOTypeItem:
			items := make([]*Item, 0, len(obj.objectData))
			c.mu.Lock()
			for _, data := range obj.objectData {
				item, err := decodeItem(data)
				if err != nil {
					c.mu.Unlock()
					c.logger.Error("tf2: decode backpack item", "err", err)
					return
				}
				cp := item
				c.cache.items[item.ID] = &cp
				items = append(items, &cp)
			}
			c.mu.Unlock()

			if c.OnBackpackLoaded != nil {
				c.OnBackpackLoaded(items)
			}

		case SOTypeAccount:
			if len(obj.objectData) == 0 {
				continue
			}
			acc, err := decodeAccount(obj.objectData[0])
			if err != nil {
				c.logger.Error("tf2: decode account", "err", err)
				return
			}
			c.mu.Lock()
			c.cache.account = &acc
			c.mu.Unlock()

			if c.OnAccountLoaded != nil {
				c.OnAccountLoaded(&acc)
			}
		}
	}
}

func (c *Client) handleSOCreate(body []byte) {
	so, err := decodeSingleObject(body)
	if err != nil {
		c.logger.Error("tf2: decode SO create", "err", err)
		return
	}
	if so.typeID != SOTypeItem {
		return
	}

	c.mu.Lock()
	if len(c.cache.items) == 0 && c.cache.account == nil {
		// Backpack not loaded yet.
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	item, err := decodeItem(so.objectData)
	if err != nil {
		c.logger.Error("tf2: decode SO create item", "err", err)
		return
	}

	cp := item
	c.mu.Lock()
	c.cache.items[item.ID] = &cp
	c.mu.Unlock()

	if c.OnItemAcquired != nil {
		c.OnItemAcquired(&cp)
	}
}

func (c *Client) handleSOUpdate(body []byte) {
	so, err := decodeSingleObject(body)
	if err != nil {
		c.logger.Error("tf2: decode SO update", "err", err)
		return
	}
	c.applySingleUpdate(so)
}

func (c *Client) handleSODestroy(body []byte) {
	so, err := decodeSingleObject(body)
	if err != nil {
		c.logger.Error("tf2: decode SO destroy", "err", err)
		return
	}
	if so.typeID != SOTypeItem {
		return
	}

	item, err := decodeItem(so.objectData)
	if err != nil {
		c.logger.Error("tf2: decode SO destroy item", "err", err)
		return
	}

	c.mu.Lock()
	old, ok := c.cache.items[item.ID]
	if ok {
		delete(c.cache.items, item.ID)
	}
	c.mu.Unlock()

	if ok && c.OnItemRemoved != nil {
		c.OnItemRemoved(old)
	}
}

func (c *Client) handleSOUpdateMultiple(body []byte) {
	objects, err := decodeMultipleObjects(body)
	if err != nil {
		c.logger.Error("tf2: decode SO update multiple", "err", err)
		return
	}
	for _, so := range objects {
		c.applySingleUpdate(so)
	}
}

func (c *Client) handleSOCacheSubscriptionCheck() {
	// Respond with CMsgSOCacheSubscriptionRefresh so the GC sends us
	// the full CacheSubscribed with backpack and account data.
	// CMsgSOCacheSubscriptionRefresh: field 1 (fixed64) = owner SteamID64.
	sid := c.cm.SteamID().ToSteamID64()
	var body []byte
	body = protowire.AppendTag(body, 1, protowire.Fixed64Type)
	body = protowire.AppendFixed64(body, sid)

	c.logger.Debug("tf2: requesting SO cache subscription refresh")
	if err := c.SendMessage(context.Background(), MsgSOCacheSubscriptionRefresh, body); err != nil {
		c.logger.Error("tf2: send SO cache subscription refresh", "err", err)
	}
}

func (c *Client) applySingleUpdate(so singleObject) {
	switch so.typeID {
	case SOTypeItem:
		c.mu.Lock()
		if len(c.cache.items) == 0 && c.cache.account == nil {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		item, err := decodeItem(so.objectData)
		if err != nil {
			c.logger.Error("tf2: decode SO update item", "err", err)
			return
		}

		cp := item
		c.mu.Lock()
		old := c.cache.items[item.ID]
		c.cache.items[item.ID] = &cp
		c.mu.Unlock()

		if c.OnItemChanged != nil {
			c.OnItemChanged(old, &cp)
		}

	case SOTypeAccount:
		acc, err := decodeAccount(so.objectData)
		if err != nil {
			c.logger.Error("tf2: decode SO update account", "err", err)
			return
		}

		c.mu.Lock()
		c.cache.account = &acc
		c.mu.Unlock()

		if c.OnAccountUpdate != nil {
			c.OnAccountUpdate(&acc)
		}
	}
}
