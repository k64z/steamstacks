package steamclient

import "fmt"

// EMsg identifies Steam CM message types.
type EMsg uint32

const (
	EMsgMulti                       EMsg = 1
	EMsgChannelEncryptRequest       EMsg = 1303
	EMsgChannelEncryptResponse      EMsg = 1304
	EMsgChannelEncryptResult        EMsg = 1305
	EMsgClientHeartBeat             EMsg = 703
	EMsgClientLogOff                EMsg = 706
	EMsgClientRemoveFriend          EMsg = 714
	EMsgClientChangeStatus          EMsg = 716
	EMsgClientFriendMsg             EMsg = 718
	EMsgClientGamesPlayed           EMsg = 742
	EMsgClientLogOnResponse         EMsg = 751
	EMsgClientLoggedOff             EMsg = 757
	EMsgClientPersonaState          EMsg = 766
	EMsgClientFriendsList           EMsg = 767
	EMsgClientAddFriend             EMsg = 791
	EMsgClientAddFriendResponse     EMsg = 792
	EMsgClientRequestFriendData     EMsg = 815
	EMsgClientSessionToken          EMsg = 850
	EMsgClientSetIgnoreFriend       EMsg = 855
	EMsgClientSetIgnoreFriendResponse EMsg = 856
	EMsgClientFriendMsgIncoming     EMsg = 5427
	EMsgClientLogon                 EMsg = 5514
	EMsgClientFriendMsgEchoToSender EMsg = 5578
	EMsgClientPersonaChangeResponse EMsg = 5584
	EMsgClientHello                 EMsg = 9805
)

const ProtoMask uint32 = 0x80000000
const ProtoVersion uint32 = 65581

var emsgNames = map[EMsg]string{
	EMsgMulti:                       "Multi",
	EMsgChannelEncryptRequest:       "ChannelEncryptRequest",
	EMsgChannelEncryptResponse:      "ChannelEncryptResponse",
	EMsgChannelEncryptResult:        "ChannelEncryptResult",
	EMsgClientHeartBeat:             "ClientHeartBeat",
	EMsgClientLogOff:                "ClientLogOff",
	EMsgClientRemoveFriend:          "ClientRemoveFriend",
	EMsgClientChangeStatus:          "ClientChangeStatus",
	EMsgClientFriendMsg:             "ClientFriendMsg",
	EMsgClientGamesPlayed:           "ClientGamesPlayed",
	EMsgClientLogOnResponse:         "ClientLogOnResponse",
	EMsgClientLoggedOff:             "ClientLoggedOff",
	EMsgClientPersonaState:          "ClientPersonaState",
	EMsgClientFriendsList:           "ClientFriendsList",
	EMsgClientAddFriend:             "ClientAddFriend",
	EMsgClientAddFriendResponse:     "ClientAddFriendResponse",
	EMsgClientRequestFriendData:     "ClientRequestFriendData",
	EMsgClientSessionToken:          "ClientSessionToken",
	EMsgClientSetIgnoreFriend:       "ClientSetIgnoreFriend",
	EMsgClientSetIgnoreFriendResponse: "ClientSetIgnoreFriendResponse",
	EMsgClientFriendMsgIncoming:     "ClientFriendMsgIncoming",
	EMsgClientLogon:                 "ClientLogon",
	EMsgClientFriendMsgEchoToSender: "ClientFriendMsgEchoToSender",
	EMsgClientPersonaChangeResponse: "ClientPersonaChangeResponse",
	EMsgClientHello:                 "ClientHello",
}

func (e EMsg) String() string {
	if name, ok := emsgNames[e]; ok {
		return name
	}
	return fmt.Sprintf("EMsg(%d)", uint32(e))
}
