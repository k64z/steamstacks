package steamsession

type EAuthSessionGuardType int32

const (
	EAuthSessionGuardTypeUnknown EAuthSessionGuardType = iota
	EAuthSessionGuardTypeNone
	EAuthSessionGuardTypeEmailCode
	EAuthSessionGuardTypeDeviceCode
	EAuthSessionGuardTypeDeviceConfirmation
	EAuthSessionGuardTypeEmailConfirmation
	EAuthSessionGuardTypeMachineToken
)

type PlatformType int32

const (
	PlatformTypeSteamClient = iota + 1
	PlatformTypeWebBrowser
	PlatformTypeMobileApp
)

type Persistence int32

const (
	PersistenceEphemereal Persistence = iota
	PersistencePersistent
)
