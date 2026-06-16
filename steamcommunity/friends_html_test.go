package steamcommunity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleFriendsPage = `
<html><body>
<div class="profile_friends_list">
  <div class="selectable friend_block_v2 persona in-game  " data-steamid="76561198000000333" data-miniprofile="40000333">
    <div class="indicator select_friend"><input class="select_friend_checkbox" type="checkbox"/></div>
    <a class="selectable_overlay" href="https://steamcommunity.com/id/sampleuser/"></a>
    <div class="player_avatar friend_block_link_overlay in-game">
      <img src="https://avatars.fastly.steamstatic.com/aabbccdd_medium.jpg"/>
    </div>
    <div class="friend_block_content">
      ¡ Sample Trader ⇄ 🪙<br/>
      <span class="friend_small_text">Non-Steam Game</span>
      <span class="friend_game_link">Non-Steam Game</span>
    </div>
  </div>
  <div class="selectable friend_block_v2 persona online" data-steamid="76561198000000111" data-miniprofile="40000111">
    <div class="player_avatar friend_block_link_overlay online">
      <img src="https://avatars.fastly.steamstatic.com/online_medium.jpg"/>
    </div>
    <div class="friend_block_content">
      OnlineFriend<br/>
      <span class="friend_small_text"></span>
      <span class="friend_game_link"></span>
    </div>
  </div>
  <div class="selectable friend_block_v2 persona offline" data-steamid="76561198000000222" data-miniprofile="40000222">
    <div class="player_avatar friend_block_link_overlay offline">
      <img src="https://avatars.fastly.steamstatic.com/off_medium.jpg"/>
    </div>
    <div class="friend_block_content">
      OfflineFriend<br/>
      <span class="friend_small_text"></span>
    </div>
  </div>
</div>
</body></html>`

func TestGetFriendsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/profiles/76561198000000000/friends"
		if r.URL.Path != want {
			t.Errorf("path = %q; want %q", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(sampleFriendsPage))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	out, err := c.GetFriendsHTML(context.Background())
	if err != nil {
		t.Fatalf("GetFriendsHTML: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d; want 3", len(out))
	}

	ingame := out[0]
	if ingame.SteamID.ToSteamID64() != 76561198000000333 {
		t.Errorf("ingame SteamID = %d; want 76561198000000333", ingame.SteamID.ToSteamID64())
	}
	if ingame.PersonaName != "¡ Sample Trader ⇄ 🪙" {
		t.Errorf("ingame PersonaName = %q", ingame.PersonaName)
	}
	if ingame.State != FriendStateInGame {
		t.Errorf("ingame State = %q; want in-game", ingame.State)
	}
	if ingame.GameName != "Non-Steam Game" {
		t.Errorf("ingame GameName = %q; want Non-Steam Game", ingame.GameName)
	}
	if !strings.Contains(ingame.AvatarURL, "aabbccdd") {
		t.Errorf("ingame AvatarURL = %q", ingame.AvatarURL)
	}

	online := out[1]
	if online.State != FriendStateOnline || online.GameName != "" {
		t.Errorf("online: state=%q game=%q", online.State, online.GameName)
	}

	offline := out[2]
	if offline.State != FriendStateOffline {
		t.Errorf("offline: state=%q", offline.State)
	}
}

const samplePendingPage = `
<html><body>
<div id="search_results" class="profile_friends search_results">
  <div class="selectable invite_row" data-steamid="76561198000000444">
    <a class="selectable_overlay invite_row_overlay" href="https://steamcommunity.com/id/received1/"></a>
    <div class="invite_row_content">
      <div class="invite_row_left">
        <div class="playerAvatar online">
          <img src="https://avatars.fastly.steamstatic.com/recv1_medium.jpg"/>
        </div>
      </div>
      <a class="invite_block_name linkTitle">Received1</a>
    </div>
  </div>
  <div class="selectable invite_row" data-steamid="76561198000000555">
    <a class="selectable_overlay invite_row_overlay" href="https://steamcommunity.com/id/received2/"></a>
    <div class="invite_row_content">
      <div class="invite_row_left">
        <div class="playerAvatar offline">
          <img src="https://avatars.fastly.steamstatic.com/recv2_medium.jpg"/>
        </div>
      </div>
      <a class="invite_block_name linkTitle">Received2</a>
    </div>
  </div>
</div>
<div class="profile_friends search_results_sentinvites">
  <div class="selectable invite_row" data-steamid="76561198000000666">
    <a class="selectable_overlay invite_row_overlay" href="https://steamcommunity.com/id/sent1/"></a>
    <div class="invite_row_content">
      <div class="invite_row_left">
        <div class="playerAvatar in-game">
          <img src="https://avatars.fastly.steamstatic.com/sent_medium.jpg"/>
        </div>
      </div>
      <a class="invite_block_name linkTitle">SentTarget</a>
    </div>
  </div>
</div>
</body></html>`

func TestGetPendingInvitesHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(samplePendingPage))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	received, sent, err := c.GetPendingInvitesHTML(context.Background())
	if err != nil {
		t.Fatalf("GetPendingInvitesHTML: %v", err)
	}
	if len(received) != 2 {
		t.Fatalf("received len = %d; want 2", len(received))
	}
	if len(sent) != 1 {
		t.Fatalf("sent len = %d; want 1", len(sent))
	}
	if received[0].PersonaName != "Received1" || received[0].State != FriendStateOnline {
		t.Errorf("received[0] = %+v", received[0])
	}
	if received[1].PersonaName != "Received2" || received[1].State != FriendStateOffline {
		t.Errorf("received[1] = %+v", received[1])
	}
	if sent[0].PersonaName != "SentTarget" || sent[0].State != FriendStateInGame {
		t.Errorf("sent[0] = %+v", sent[0])
	}
}

func TestGetFriendsHTML_StateClasses(t *testing.T) {
	cases := []struct {
		classes string
		want    FriendState
	}{
		{"selectable friend_block_v2 persona online", FriendStateOnline},
		{"selectable friend_block_v2 persona offline", FriendStateOffline},
		{"selectable friend_block_v2 persona in-game  ", FriendStateInGame},
		{"selectable friend_block_v2 persona away", FriendStateAway},
		{"selectable friend_block_v2 persona busy", FriendStateBusy},
		{"selectable friend_block_v2 persona snooze", FriendStateSnooze},
		{"selectable friend_block_v2 persona looking-to-trade", FriendStateLookingToTrade},
		{"selectable friend_block_v2 persona looking-to-play", FriendStateLookingToPlay},
		{"selectable friend_block_v2 persona random-unknown", FriendStateOffline},
	}
	for _, tc := range cases {
		if got := friendStateFromClasses(tc.classes); got != tc.want {
			t.Errorf("friendStateFromClasses(%q) = %q; want %q", tc.classes, got, tc.want)
		}
	}
}
