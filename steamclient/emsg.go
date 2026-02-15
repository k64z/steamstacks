package steamclient

import "fmt"

// EMsg identifies Steam CM message types.
type EMsg uint32

const (
	EMsgMulti                  EMsg = 1
	EMsgChannelEncryptRequest  EMsg = 1303
	EMsgChannelEncryptResponse EMsg = 1304
	EMsgChannelEncryptResult   EMsg = 1305
	EMsgClientHeartBeat        EMsg = 703
	EMsgClientLogOff           EMsg = 706
	EMsgClientLogOnResponse    EMsg = 751
	EMsgClientLoggedOff        EMsg = 757
	EMsgClientSessionToken     EMsg = 850
	EMsgClientLogon            EMsg = 5514
	EMsgClientHello            EMsg = 9805
)

const ProtoMask uint32 = 0x80000000
const ProtoVersion uint32 = 65581

var emsgNames = map[EMsg]string{
	EMsgMulti:                  "Multi",
	EMsgChannelEncryptRequest:  "ChannelEncryptRequest",
	EMsgChannelEncryptResponse: "ChannelEncryptResponse",
	EMsgChannelEncryptResult:   "ChannelEncryptResult",
	EMsgClientHeartBeat:        "ClientHeartBeat",
	EMsgClientLogOff:           "ClientLogOff",
	EMsgClientLogOnResponse:    "ClientLogOnResponse",
	EMsgClientLoggedOff:        "ClientLoggedOff",
	EMsgClientSessionToken:     "ClientSessionToken",
	EMsgClientLogon:            "ClientLogon",
	EMsgClientHello:            "ClientHello",
}

func (e EMsg) String() string {
	if name, ok := emsgNames[e]; ok {
		return name
	}
	return fmt.Sprintf("EMsg(%d)", uint32(e))
}
