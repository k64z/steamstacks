package steamcommunity

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/k64z/steamstacks/steamapi"
	"github.com/k64z/steamstacks/steamid"
	"golang.org/x/net/html"
)

// FriendState mirrors the persona-state class Steam renders on each
// friend_block_v2 element. Values are the exact CSS modifier strings —
// keep them stable so the frontend can switch on them directly.
type FriendState string

const (
	FriendStateOffline        FriendState = "offline"
	FriendStateOnline         FriendState = "online"
	FriendStateAway           FriendState = "away"
	FriendStateBusy           FriendState = "busy"
	FriendStateSnooze         FriendState = "snooze"
	FriendStateInGame         FriendState = "in-game"
	FriendStateLookingToTrade FriendState = "looking-to-trade"
	FriendStateLookingToPlay  FriendState = "looking-to-play"
)

// FriendBlock is a single row scraped from one of steamcommunity's
// /friends/* HTML pages. Mirrors what Steam Web renders, so consumers
// don't need a Steam Web API key or a CM connection to populate a
// friends UI.
type FriendBlock struct {
	SteamID     steamid.SteamID
	PersonaName string
	AvatarURL   string // _medium-sized URL
	State       FriendState
	GameName    string // when in-game; may also hold a non-Steam game name
}

// GetFriendsHTML fetches the logged-in account's /friends/ page and
// returns one FriendBlock per actual friend. Use GetPendingInvitesHTML
// and GetBlockedHTML for the other relationship categories.
func (c *Community) GetFriendsHTML(ctx context.Context) ([]FriendBlock, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	return c.fetchFriendBlocks(ctx, c.friendsPageURL(""))
}

// GetPendingInvitesHTML fetches /friends/pending and partitions the
// rendered invite rows into received vs sent.
//
// Steam renders the pending page as two siblings of #search_results:
//   - <div id="search_results" class="profile_friends search_results">       — received
//   - <div class="profile_friends search_results_sentinvites">                — sent
//
// Each contains one .invite_row per pending invite (with data-steamid +
// a nested .playerAvatar.<state> indicator). The DOM differs from
// /friends/ which uses .friend_block_v2 — invite_row carries less
// metadata (no game name; less reliable level + state) but is sufficient
// for the UI's accept/ignore/cancel flow.
func (c *Community) GetPendingInvitesHTML(ctx context.Context) (received, sent []FriendBlock, err error) {
	if err := c.ensureInit(); err != nil {
		return nil, nil, err
	}
	doc, err := c.fetchHTML(ctx, c.friendsPageURL("pending"))
	if err != nil {
		return nil, nil, err
	}
	received = parseInviteRowsIn(doc, "search_results", "")
	sent = parseInviteRowsIn(doc, "", "search_results_sentinvites")
	return received, sent, nil
}

// GetBlockedHTML fetches /friends/blocked.
func (c *Community) GetBlockedHTML(ctx context.Context) ([]FriendBlock, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	return c.fetchFriendBlocks(ctx, c.friendsPageURL("blocked"))
}

func (c *Community) friendsPageURL(section string) string {
	base := fmt.Sprintf("https://steamcommunity.com/profiles/%d/friends", c.SteamID.ToSteamID64())
	if section == "" {
		return base
	}
	return base + "/" + section
}

func (c *Community) fetchFriendBlocks(ctx context.Context, url string) ([]FriendBlock, error) {
	doc, err := c.fetchHTML(ctx, url)
	if err != nil {
		return nil, err
	}
	return parseFriendBlocks(doc), nil
}

func (c *Community) fetchHTML(ctx context.Context, url string) (*html.Node, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, steamapi.HTTPStatusError(resp.StatusCode, body)
	}
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}

// parseFriendBlocks walks the document and emits a FriendBlock for every
// <div class="friend_block_v2 ..."> it finds. State is derived from the
// CSS modifier class on the block (e.g. "online", "in-game"); avatar +
// game name are pulled from the nested elements Steam consistently
// renders inside.
func parseFriendBlocks(doc *html.Node) []FriendBlock {
	var out []FriendBlock
	walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "div" {
			return true
		}
		classes := attr(n, "class")
		if !hasClass(classes, "friend_block_v2") {
			return true
		}
		b := blockFromNode(n, classes)
		if b.SteamID != 0 {
			out = append(out, b)
		}
		// Don't descend — block contents are inspected by blockFromNode.
		return false
	})
	return out
}

// parseInviteRowsIn finds the container element identified by `id` or
// `cls` and returns one FriendBlock per .invite_row descendant. Either
// id or cls may be empty — the first match wins. Returns nil when the
// container isn't present (e.g. account has no sent invites).
func parseInviteRowsIn(doc *html.Node, id, cls string) []FriendBlock {
	var container *html.Node
	walk(doc, func(n *html.Node) bool {
		if container != nil {
			return false
		}
		if n.Type != html.ElementNode {
			return true
		}
		if id != "" && attr(n, "id") == id {
			container = n
			return false
		}
		if cls != "" && hasClass(attr(n, "class"), cls) {
			container = n
			return false
		}
		return true
	})
	if container == nil {
		return nil
	}
	var out []FriendBlock
	walk(container, func(n *html.Node) bool {
		if n == container || n.Type != html.ElementNode || n.Data != "div" {
			return true
		}
		if !hasClass(attr(n, "class"), "invite_row") {
			return true
		}
		b := inviteRowToBlock(n)
		if b.SteamID != 0 {
			out = append(out, b)
		}
		return false
	})
	return out
}

// inviteRowToBlock extracts persona + avatar + state from an .invite_row.
// The DOM is sparser than friend_block_v2: no data-miniprofile, no
// .friend_block_content / .friend_game_link, and the persona state
// lives on a nested .playerAvatar.<state> div.
func inviteRowToBlock(n *html.Node) FriendBlock {
	b := FriendBlock{State: FriendStateOffline}
	if sid := attr(n, "data-steamid"); sid != "" {
		if parsed, err := steamid.FromString(sid); err == nil {
			b.SteamID = parsed
		}
	}
	walk(n, func(c *html.Node) bool {
		if c == n || c.Type != html.ElementNode {
			return true
		}
		switch c.Data {
		case "img":
			if b.AvatarURL == "" {
				if src := attr(c, "src"); strings.Contains(src, "avatars") {
					b.AvatarURL = src
				}
			}
		case "div", "span":
			class := attr(c, "class")
			if hasClass(class, "playerAvatar") {
				b.State = friendStateFromClasses(class)
			}
			if b.PersonaName == "" && hasClass(class, "invite_block_name") {
				b.PersonaName = strings.TrimSpace(innerText(c))
			}
		case "a":
			if b.PersonaName == "" && hasClass(attr(c, "class"), "linkTitle") {
				b.PersonaName = strings.TrimSpace(innerText(c))
			}
		}
		return true
	})
	return b
}

func blockFromNode(n *html.Node, classes string) FriendBlock {
	b := FriendBlock{
		State: friendStateFromClasses(classes),
	}
	if sid := attr(n, "data-steamid"); sid != "" {
		if parsed, err := steamid.FromString(sid); err == nil {
			b.SteamID = parsed
		}
	}
	walk(n, func(child *html.Node) bool {
		if child == n || child.Type != html.ElementNode {
			return true
		}
		switch child.Data {
		case "img":
			if b.AvatarURL == "" {
				if src := attr(child, "src"); strings.Contains(src, "avatars") {
					b.AvatarURL = src
				}
			}
		case "div":
			if hasClass(attr(child, "class"), "friend_block_content") {
				b.PersonaName = personaNameFromContent(child)
			}
		case "span":
			if b.GameName == "" && hasClass(attr(child, "class"), "friend_game_link") {
				b.GameName = strings.TrimSpace(innerText(child))
			}
		}
		return true
	})
	return b
}

// personaNameFromContent extracts the persona name (first text node
// before the <br>) from the friend_block_content div. Trims surrounding
// whitespace and collapses any inner whitespace into single spaces.
func personaNameFromContent(n *html.Node) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			s := strings.TrimSpace(c.Data)
			if s != "" {
				return s
			}
		}
		if c.Type == html.ElementNode && c.Data == "br" {
			break
		}
	}
	return ""
}

// friendStateFromClasses scans the friend_block_v2 div's class attribute
// for the persona-state CSS modifier and maps it to a typed FriendState.
// Anything unrecognised falls back to Offline so the UI doesn't render
// stale "online" badges.
func friendStateFromClasses(classes string) FriendState {
	for _, c := range strings.Fields(classes) {
		switch c {
		case "online":
			return FriendStateOnline
		case "offline":
			return FriendStateOffline
		case "in-game":
			return FriendStateInGame
		case "away":
			return FriendStateAway
		case "busy":
			return FriendStateBusy
		case "snooze":
			return FriendStateSnooze
		case "looking-to-trade":
			return FriendStateLookingToTrade
		case "looking-to-play":
			return FriendStateLookingToPlay
		}
	}
	return FriendStateOffline
}

func hasClass(classes, want string) bool {
	for _, c := range strings.Fields(classes) {
		if c == want {
			return true
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func innerText(n *html.Node) string {
	var sb strings.Builder
	walk(n, func(c *html.Node) bool {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
		return true
	})
	return sb.String()
}

// walk runs fn against n and every descendant. fn returning false stops
// the descent into n's children but doesn't abort the wider walk.
func walk(n *html.Node, fn func(*html.Node) bool) {
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
