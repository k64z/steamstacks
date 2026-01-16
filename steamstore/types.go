package steamstore

import "fmt"

// EResult represents Steam API result codes
type EResult int

const (
	EResultOK                 EResult = 1
	EResultFail               EResult = 2
	EResultNoConnection       EResult = 3
	EResultInvalidPassword    EResult = 5
	EResultLoggedInElsewhere  EResult = 6
	EResultInvalidProtocol    EResult = 7
	EResultInvalidParam       EResult = 8
	EResultFileNotFound       EResult = 9
	EResultBusy               EResult = 10
	EResultInvalidState       EResult = 11
	EResultInvalidName        EResult = 12
	EResultInvalidEmail       EResult = 13
	EResultDuplicateName      EResult = 14
	EResultAccessDenied       EResult = 15
	EResultTimeout            EResult = 16
	EResultBanned             EResult = 17
	EResultAccountNotFound    EResult = 18
	EResultInvalidSteamID     EResult = 19
	EResultServiceUnavailable EResult = 20
	EResultNotLoggedOn        EResult = 21
	EResultPending            EResult = 22
	EResultLimitExceeded      EResult = 25
	EResultRevoked            EResult = 26
	EResultExpired            EResult = 27
	EResultAlreadyRedeemed    EResult = 28
	EResultDuplicateRequest   EResult = 29
	EResultAlreadyOwned       EResult = 30
	EResultIPNotFound         EResult = 31
	EResultPersistFailed      EResult = 32
	EResultLockingFailed      EResult = 33
	EResultLogonSessionReplaced EResult = 34
	EResultConnectFailed        EResult = 35
	EResultHandshakeFailed      EResult = 36
	EResultIOFailure            EResult = 37
	EResultRemoteDisconnect     EResult = 38
	EResultRateLimitExceeded    EResult = 84
	EResultAccountDisabled      EResult = 85
	EResultAccountLockedDown    EResult = 105
)

func (e EResult) String() string {
	switch e {
	case EResultOK:
		return "OK"
	case EResultFail:
		return "Fail"
	case EResultNoConnection:
		return "NoConnection"
	case EResultInvalidPassword:
		return "InvalidPassword"
	case EResultLoggedInElsewhere:
		return "LoggedInElsewhere"
	case EResultInvalidProtocol:
		return "InvalidProtocol"
	case EResultInvalidParam:
		return "InvalidParam"
	case EResultFileNotFound:
		return "FileNotFound"
	case EResultBusy:
		return "Busy"
	case EResultInvalidState:
		return "InvalidState"
	case EResultInvalidName:
		return "InvalidName"
	case EResultInvalidEmail:
		return "InvalidEmail"
	case EResultDuplicateName:
		return "DuplicateName"
	case EResultAccessDenied:
		return "AccessDenied"
	case EResultTimeout:
		return "Timeout"
	case EResultBanned:
		return "Banned"
	case EResultAccountNotFound:
		return "AccountNotFound"
	case EResultInvalidSteamID:
		return "InvalidSteamID"
	case EResultServiceUnavailable:
		return "ServiceUnavailable"
	case EResultNotLoggedOn:
		return "NotLoggedOn"
	case EResultPending:
		return "Pending"
	case EResultLimitExceeded:
		return "LimitExceeded"
	case EResultRevoked:
		return "Revoked"
	case EResultExpired:
		return "Expired"
	case EResultAlreadyRedeemed:
		return "AlreadyRedeemed"
	case EResultDuplicateRequest:
		return "DuplicateRequest"
	case EResultAlreadyOwned:
		return "AlreadyOwned"
	case EResultIPNotFound:
		return "IPNotFound"
	case EResultPersistFailed:
		return "PersistFailed"
	case EResultLockingFailed:
		return "LockingFailed"
	case EResultLogonSessionReplaced:
		return "LogonSessionReplaced"
	case EResultConnectFailed:
		return "ConnectFailed"
	case EResultHandshakeFailed:
		return "HandshakeFailed"
	case EResultIOFailure:
		return "IOFailure"
	case EResultRemoteDisconnect:
		return "RemoteDisconnect"
	case EResultRateLimitExceeded:
		return "RateLimitExceeded"
	case EResultAccountDisabled:
		return "AccountDisabled"
	case EResultAccountLockedDown:
		return "AccountLockedDown"
	default:
		return fmt.Sprintf("EResult(%d)", e)
	}
}

// EPurchaseResult represents purchase/wallet operation result codes
type EPurchaseResult int

const (
	EPurchaseResultOK                              EPurchaseResult = 0
	EPurchaseResultAlreadyPurchased                EPurchaseResult = 9
	EPurchaseResultRegionNotSupported              EPurchaseResult = 13
	EPurchaseResultBadActivationCode               EPurchaseResult = 14
	EPurchaseResultDuplicateActivationCode         EPurchaseResult = 15
	EPurchaseResultUseOtherPaymentMethod           EPurchaseResult = 16
	EPurchaseResultUseOtherFunctionSource          EPurchaseResult = 17
	EPurchaseResultInvalidPackage                  EPurchaseResult = 18
	EPurchaseResultInvalidPaymentMethod            EPurchaseResult = 19
	EPurchaseResultInvalidData                     EPurchaseResult = 20
	EPurchaseResultOthersBeingPurchased            EPurchaseResult = 21
	EPurchaseResultRestrictedCountry               EPurchaseResult = 22
	EPurchaseResultPaymentNotWhitelisted           EPurchaseResult = 24
	EPurchaseResultGiftWalletCountryMismatch       EPurchaseResult = 25
	EPurchaseResultInsufficientFunds               EPurchaseResult = 26
	EPurchaseResultContactSupport                  EPurchaseResult = 27
	EPurchaseResultProductOnCooldown               EPurchaseResult = 53
	EPurchaseResultPendingApproval                 EPurchaseResult = 54
	EPurchaseResultTooManyActivationsForAccount    EPurchaseResult = 60
	EPurchaseResultTooManyActivationsForMachine    EPurchaseResult = 61
	EPurchaseResultBaseGameRequired                EPurchaseResult = 62
	EPurchaseResultDoesNotOwnRequiredApp           EPurchaseResult = 71
	EPurchaseResultWalletCurrencyMismatch          EPurchaseResult = 73
)

func (e EPurchaseResult) String() string {
	switch e {
	case EPurchaseResultOK:
		return "OK"
	case EPurchaseResultAlreadyPurchased:
		return "AlreadyPurchased"
	case EPurchaseResultRegionNotSupported:
		return "RegionNotSupported"
	case EPurchaseResultBadActivationCode:
		return "BadActivationCode"
	case EPurchaseResultDuplicateActivationCode:
		return "DuplicateActivationCode"
	case EPurchaseResultUseOtherPaymentMethod:
		return "UseOtherPaymentMethod"
	case EPurchaseResultUseOtherFunctionSource:
		return "UseOtherFunctionSource"
	case EPurchaseResultInvalidPackage:
		return "InvalidPackage"
	case EPurchaseResultInvalidPaymentMethod:
		return "InvalidPaymentMethod"
	case EPurchaseResultInvalidData:
		return "InvalidData"
	case EPurchaseResultOthersBeingPurchased:
		return "OthersBeingPurchased"
	case EPurchaseResultRestrictedCountry:
		return "RestrictedCountry"
	case EPurchaseResultPaymentNotWhitelisted:
		return "PaymentNotWhitelisted"
	case EPurchaseResultGiftWalletCountryMismatch:
		return "GiftWalletCountryMismatch"
	case EPurchaseResultInsufficientFunds:
		return "InsufficientFunds"
	case EPurchaseResultContactSupport:
		return "ContactSupport"
	case EPurchaseResultProductOnCooldown:
		return "ProductOnCooldown"
	case EPurchaseResultPendingApproval:
		return "PendingApproval"
	case EPurchaseResultTooManyActivationsForAccount:
		return "TooManyActivationsForAccount"
	case EPurchaseResultTooManyActivationsForMachine:
		return "TooManyActivationsForMachine"
	case EPurchaseResultBaseGameRequired:
		return "BaseGameRequired"
	case EPurchaseResultDoesNotOwnRequiredApp:
		return "DoesNotOwnRequiredApp"
	case EPurchaseResultWalletCurrencyMismatch:
		return "WalletCurrencyMismatch"
	default:
		return fmt.Sprintf("EPurchaseResult(%d)", e)
	}
}

// StoreError represents an error from Steam Store operations
type StoreError struct {
	Result         EResult
	PurchaseResult EPurchaseResult
	Message        string
}

func (e *StoreError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.PurchaseResult != EPurchaseResultOK {
		return fmt.Sprintf("purchase error: %s", e.PurchaseResult)
	}
	return fmt.Sprintf("store error: %s", e.Result)
}

func NewStoreError(result EResult, message string) *StoreError {
	return &StoreError{
		Result:  result,
		Message: message,
	}
}

func NewPurchaseError(purchaseResult EPurchaseResult, message string) *StoreError {
	return &StoreError{
		PurchaseResult: purchaseResult,
		Message:        message,
	}
}
