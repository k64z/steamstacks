package steamapi

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
	SentOffers     []TradeOffer `json:"trade_offers_sent"`
	ReceivedOffers []TradeOffer `json:"trade_offers_received"`
}
