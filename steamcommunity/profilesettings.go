package steamcommunity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type PrivacyOption int

const (
	PrivacyOptionPrivate     PrivacyOption = 1
	PrivacyOptionFriendsOnly PrivacyOption = 2
	PrivacyOptionPublic      PrivacyOption = 3
)

type ProfileData struct {
	Config     *ProfileConfig
	EditConfig *ProfileEditConfig
	Badges     *ProfileBadges
	RawConfig  string // raw JSON (after HTML-unescape), useful for debugging
	RawEdit    string
	RawBadges  string
}

type PrivacySettings struct {
	PrivacyProfile        PrivacyOption `json:"PrivacyProfile"`
	PrivacyInventory      PrivacyOption `json:"PrivacyInventory"`
	PrivacyInventoryGifts PrivacyOption `json:"PrivacyInventoryGifts"`
	PrivacyOwnedGames     PrivacyOption `json:"PrivacyOwnedGames"`
	PrivacyPlaytime       PrivacyOption `json:"PrivacyPlaytime"`
	PrivacyFriendsList    PrivacyOption `json:"PrivacyFriendsList"`
}

type ProfileConfig struct {
	ProfileURL string `json:"ProfileURL"`
}

type ProfileEditConfig struct {
	PersonaName               string `json:"strPersonaName"`
	FilteredPersonaName       string `json:"strFilteredPersonaName"`
	CustomURL                 string `json:"strCustomURL"`
	RealName                  string `json:"strRealName"`
	FilteredRealName          string `json:"strFilteredRealName"`
	Summary                   string `json:"strSummary"`
	AvatarHash                string `json:"strAvatarHash"`
	PersonaNameBannedUntil    int64  `json:"rtPersonaNameBannedUntil"` // rt* is probably "realtime" (timestamps)
	ProfileSummaryBannedUntil int64  `json:"rtProfileSummaryBannedUntil"`
	AvatarBannedUntil         int64  `json:"rtAvatarBannedUntil"`
	LocationData              struct {
		LocCountry     string `json:"locCountry"`
		LocCountryCode string `json:"locCountryCode"`
		LocState       string `json:"locState"`
		LocStateCode   string `json:"locStateCode"`
		LocCity        string `json:"locCity"`
		LocCityCode    string `json:"locCityCode"`
	} `json:"LocationData"`
	ActiveTheme struct {
		ThemeID string `json:"theme_id"`
		Title   string `json:"title"`
	} `json:"ActiveTheme"`
	ProfilePreferences struct {
		HideProfileAwards int `json:"hide_profile_awards"`
	} `json:"ProfilePreferences"`
	AvailableThemes []struct {
		ThemeID string `json:"theme_id"`
		Title   string `json:"title"`
	} `json:"rgAvailableThemes"` // Referenced Generic array
	GoldenProfileData []struct {
		AppID                 int               `json:"appid"`
		CSSURL                string            `json:"css_url"`
		FrameURL              *string           `json:"frame_url"`
		MiniprofileBackground *string           `json:"miniprofile_background"`
		MiniprofileMovie      map[string]string `json:"miniprofile_movie"`
	} `json:"rgGoldenProfileData"`
	Privacy struct {
		PrivacySettings    PrivacySettings `json:"PrivacySettings"`
		ECommentPermission int             `json:"eCommentPermission"`
	} `json:"Privacy"`
}

type ProfileBadges struct {
	Badges        map[string]Badge `json:"rgBadges"`
	FavoriteBadge struct {
		BadgeID         json.RawMessage `json:"badgeid"`
		CommunutyItemID string          `json:"communityitemid"`
	}
}

// Mixed types -> keep RawMessage for badgeid
type Badge struct {
	BadgeID         json.RawMessage `json:"badgeid"` // number or ""
	Icon            string          `json:"icon"`
	Name            string          `json:"name"`
	XP              string          `json:"xp"`
	CommunityItemID string          `json:"communityitemid"`
	ItemType        *int            `json:"item_type"`
	AppID           *int            `json:"appid"`
	BorderColor     *int            `json:"border_color"`
}

func (c *Community) ProfileData() (*ProfileData, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://steamcommunity.com/profiles/%d/edit/info", c.SteamID)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}

	profileData, err := parseProfileData(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	defer resp.Body.Close()

	return profileData, nil
}

func (c *Community) PrivacySettings() (*PrivacySettings, error) {
	profileData, err := c.ProfileData()
	if err != nil {
		return nil, fmt.Errorf("load profile data: %w", err)
	}
	return &profileData.EditConfig.Privacy.PrivacySettings, nil
}

func (c *Community) SetPrivacySettings(ctx context.Context, settings *PrivacySettings) error {
	if err := c.ensureInit(); err != nil {
		return err
	}
	if settings == nil {
		return errors.New("settings cannot be nil")
	}

	jsonData, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	formData := &url.Values{
		"sessionid":          {c.sessionID},
		"Privacy":            {string(jsonData)},
		"eCommentPermission": {"0"},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://steamcommunity.com/profiles/%d/ajaxsetprivacy/", c.SteamID),
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Success int `json:"success"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	if result.Success != 1 {
		return fmt.Errorf("unexpected success value: %d", result.Success)
	}

	return nil
}

type EditProfileRequest struct {
	PersonaName       string
	RealName          string
	CustomURL         string
	Country           string
	State             string
	City              string
	Summary           string
	HideProfileAwards string // 1 or 0
	WebLinks          []struct {
		Title string
		URL   string
	} // up to 3; will be written as weblink_{i}_title/url
}

func (c *Community) EditProfile(ctx context.Context, p EditProfileRequest) error {
	if err := c.ensureInit(); err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	w.WriteField("sessionID", c.sessionID)
	w.WriteField("type", "profileSave")
	w.WriteField("json", "1")
	w.WriteField("personaName", p.PersonaName)
	w.WriteField("real_name", p.RealName)
	w.WriteField("customURL", p.CustomURL)
	w.WriteField("country", p.Country)
	w.WriteField("state", p.State)
	w.WriteField("city", p.City)
	w.WriteField("summary", p.Summary)
	w.WriteField("hide_profile_awards", p.HideProfileAwards)

	for i, link := range p.WebLinks {
		w.WriteField("weblink_"+strconv.Itoa(i+1)+"_title", link.Title)
		w.WriteField("weblink_"+strconv.Itoa(i+1)+"_url", link.URL)
	}

	w.Close()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://steamcommunity.com/profiles/%d/edit/info", c.SteamID),
		buf,
	)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Success int    `json:"success"`
		ErrMsg  string `json:"errmsg"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	if result.Success != 1 {
		return fmt.Errorf("unexpected success value: %d", result.Success)
	}

	return nil
}

func (c *Community) UploadAvatar(ctx context.Context, avatar io.Reader) error {
	if err := c.ensureInit(); err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	{
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="avatar"; filename="blob"`)
		h.Set("Content-Type", "image/png")

		part, err := w.CreatePart(h)
		if err != nil {
			return fmt.Errorf("create avatar part: %w", err)
		}
		if _, err := io.Copy(part, avatar); err != nil {
			return fmt.Errorf("write avatar content: %w", err)
		}
	}

	fields := map[string]string{
		"type":      "player_avatar_image",
		"sId":       strconv.FormatUint(c.SteamID.ToSteamID64(), 10),
		"sessionid": c.sessionID,
		"doSub":     "1",
		"json":      "1",
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return fmt.Errorf("write field %q: %w", k, err)
		}
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close multipart body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://steamcommunity.com/actions/FileUploader/",
		buf,
	)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("unexpected success = false")
	}

	return nil
}

func (c *Community) ClearAliasHistory(ctx context.Context) error {
	if err := c.ensureInit(); err != nil {
		return err
	}
	formData := &url.Values{
		"sessionid": {c.sessionID},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://steamcommunity.com/profiles/%d/ajaxclearaliashistory/", c.SteamID),
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Success int `json:"success"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	if result.Success != 1 {
		return fmt.Errorf("unexpected success value: %d", result.Success)
	}

	return nil
}

func parseProfileData(r io.Reader) (*ProfileData, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	htmlStr := string(buf)

	var reProfileConfig = regexp.MustCompile(`(?is)<div[^>]*\bid\s*=\s*["']profile_config["'][^>]*\bdata-config\s*=\s*["'](.*?)["'][^>]*>`)
	var reProfileEdit = regexp.MustCompile(`(?is)<div[^>]*\bid\s*=\s*["']profile_edit_config["'][^>]*\bdata-profile-edit\s*=\s*["'](.*?)["'][^>]*>`)
	var reProfileBadges = regexp.MustCompile(`(?is)<div[^>]*\bid\s*=\s*["']profile_edit_config["'][^>]*\bdata-profile-badges\s*=\s*["'](.*?)["'][^>]*>`)
	var out ProfileData

	// 1) dota-config on #profile_config
	if m := reProfileConfig.FindStringSubmatch(htmlStr); len(m) == 2 {
		out.RawConfig = html.UnescapeString(m[1])
		var v ProfileConfig
		err := json.Unmarshal([]byte(out.RawConfig), &v)
		if err != nil {
			return nil, fmt.Errorf("unmarshal data-config: %w", err)
		}
		out.Config = &v
	}

	if m := reProfileEdit.FindStringSubmatch(htmlStr); len(m) == 2 {
		out.RawEdit = html.UnescapeString(m[1])
		var v ProfileEditConfig
		err := json.Unmarshal([]byte(out.RawEdit), &v)
		if err != nil {
			return nil, fmt.Errorf("unmarshal data-profile-edit: %w", err)
		}
		out.EditConfig = &v
	}

	if m := reProfileBadges.FindStringSubmatch(htmlStr); len(m) == 2 {
		out.RawBadges = html.UnescapeString(m[1])
		var v ProfileBadges
		err := json.Unmarshal([]byte(out.RawBadges), &v)
		if err != nil {
			return nil, fmt.Errorf("unmarshal data-profile-badges: %w", err)
		}
		out.Badges = &v
	}

	if out.Config == nil && out.EditConfig == nil && out.Badges == nil {
		return nil, errors.New("no data parsed")
	}

	return &out, nil
}
