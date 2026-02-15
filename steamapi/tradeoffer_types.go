package steamapi

import "strconv"

// ETradeOfferState represents the state of a trade offer
type ETradeOfferState int

const (
	ETradeOfferStateInvalid                  ETradeOfferState = 1
	ETradeOfferStateActive                   ETradeOfferState = 2
	ETradeOfferStateAccepted                 ETradeOfferState = 3
	ETradeOfferStateCountered                ETradeOfferState = 4
	ETradeOfferStateExpired                  ETradeOfferState = 5
	ETradeOfferStateCanceled                 ETradeOfferState = 6
	ETradeOfferStateDeclined                 ETradeOfferState = 7
	ETradeOfferStateInvalidItems             ETradeOfferState = 8
	ETradeOfferStateCreatedNeedsConfirmation ETradeOfferState = 9
	ETradeOfferStateCanceledBySecondFactor   ETradeOfferState = 10
	ETradeOfferStateInEscrow                 ETradeOfferState = 11
)

// EConfirmationMethod represents how a trade offer should be confirmed
type EConfirmationMethod int

const (
	EConfirmationMethodNone      EConfirmationMethod = 0
	EConfirmationMethodEmail     EConfirmationMethod = 1
	EConfirmationMethodMobileApp EConfirmationMethod = 2
)

// TradeOffer represents a Steam trade offer (CEcon_TradeOffer)
type TradeOffer struct {
	ID                 string              `json:"tradeofferid"`
	PartnerAccountID   uint32              `json:"accountid_other"`
	Message            string              `json:"message"`
	ExpirationTime     int64               `json:"expiration_time"`
	State              ETradeOfferState    `json:"trade_offer_state"`
	ItemsToGive        []TradeAsset        `json:"items_to_give"`
	ItemsToReceive     []TradeAsset        `json:"items_to_receive"`
	IsOurOffer         bool                `json:"is_our_offer"`
	TimeCreated        int64               `json:"time_created"`
	TimeUpdated        int64               `json:"time_updated"`
	FromRealTimeTrade  bool                `json:"from_real_time_trade"`
	EscrowEndDate      int64               `json:"escrow_end_date"`
	ConfirmationMethod EConfirmationMethod `json:"confirmation_method"`
}

// TradeAsset represents an item in a trade (CEcon_Asset)
type TradeAsset struct {
	AppID      int    `json:"appid"`
	ContextID  string `json:"contextid"`
	AssetID    string `json:"assetid"`
	CurrencyID string `json:"currencyid,omitempty"`
	ClassID    string `json:"classid"`
	InstanceID string `json:"instanceid"`
	Amount     string `json:"amount"`
	Missing    bool   `json:"missing,omitempty"`
}

// DescriptionKey returns the lookup key for this asset's description
// in TradeOffersResponse.Descriptions.
func (a TradeAsset) DescriptionKey() string {
	return AssetDescriptionKey(a.AppID, a.ClassID, a.InstanceID)
}

// AssetDescriptionKey builds the map key used in TradeOffersResponse.Descriptions.
// Trade offers span multiple apps, so the key includes appID unlike inventory descriptions.
func AssetDescriptionKey(appID int, classID, instanceID string) string {
	return strconv.Itoa(appID) + "_" + classID + "_" + instanceID
}

// AssetDescription describes an item returned by GetTradeOffers
// when GetDescriptions is true.
type AssetDescription struct {
	AppID          int               `json:"appid"`
	ClassID        string            `json:"classid"`
	InstanceID     string            `json:"instanceid"`
	Name           string            `json:"name"`
	MarketHashName string            `json:"market_hash_name"`
	Type           string            `json:"type"`
	Tradable       bool              `json:"tradable"`       // Steam changed wire format from int (0/1) to bool
	Marketable     bool              `json:"marketable"`     // Steam changed wire format from int (0/1) to bool
	Commodity      bool              `json:"commodity"`      // Steam changed wire format from int (0/1) to bool
	IconURL        string            `json:"icon_url"`
	IconURLLarge   string            `json:"icon_url_large,omitzero"`
	Descriptions   []DescriptionLine `json:"descriptions,omitzero"`
	Tags           []Tag             `json:"tags,omitzero"`
	Actions        []Action          `json:"actions,omitzero"`
	FraudWarnings  []string          `json:"fraudwarnings,omitzero"`
}

// DescriptionLine is a single line inside an item description block.
type DescriptionLine struct {
	Type  string `json:"type,omitzero"`
	Value string `json:"value"`
	Color string `json:"color,omitzero"`
}

// Tag is a category tag on an item description.
type Tag struct {
	Category              string `json:"category"`
	InternalName          string `json:"internal_name"`
	LocalizedCategoryName string `json:"localized_category_name"`
	LocalizedTagName      string `json:"localized_tag_name"`
	Color                 string `json:"color,omitzero"`
}

// Action is an action link on an item description.
type Action struct {
	Link string `json:"link"`
	Name string `json:"name"`
}

// GetTradeOffersOptions contains options for GetTradeOffers
type GetTradeOffersOptions struct {
	GetSentOffers        bool
	GetReceivedOffers    bool
	GetDescriptions      bool
	ActiveOnly           bool
	HistoricalOnly       bool
	Language             string
	TimeHistoricalCutoff int64
}

// TradeOffersResponse contains the response from GetTradeOffers
type TradeOffersResponse struct {
	SentOffers     []TradeOffer                `json:"trade_offers_sent"`
	ReceivedOffers []TradeOffer                `json:"trade_offers_received"`
	Descriptions   map[string]AssetDescription `json:"-"`
}

// GetTradeOfferResult contains a single trade offer with optional descriptions.
type GetTradeOfferResult struct {
	Offer        *TradeOffer
	Descriptions map[string]AssetDescription
}
