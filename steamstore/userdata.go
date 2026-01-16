package steamstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// UserData represents the response from /dynamicstore/userdata/
type UserData struct {
	OwnedApps          []int          `json:"rgOwnedApps"`
	OwnedPackages      []int          `json:"rgOwnedPackages"`
	WishlistedApps     []int          `json:"rgWishlist"`
	IgnoredApps        map[string]int `json:"rgIgnoredApps"`
	IgnoredPackages    []int          `json:"rgIgnoredPackages"`
	FollowedApps       []int          `json:"rgFollowedApps"`
	RecommendedTags    []SuggestedTag `json:"rgRecommendedTags"`
	RecommendedApps    []int          `json:"rgRecommendedApps"`
	CuratorsIgnored    []int          `json:"rgCuratorsIgnored"`
	CuratorsFollowed   []int          `json:"rgCuratorsFollowed"`
	CreatorsFollowed   []int          `json:"rgCreatorsFollowed"`
	CreatorsIgnored    []int          `json:"rgCreatorsIgnored"`
	ExcludedTags       []int          `json:"rgExcludedTags"`
	PrimaryLanguage    int            `json:"rgPrimaryLanguage"`
	SecondaryLanguages []int          `json:"rgSecondaryLanguages"`
}

// SuggestedTag represents a suggested tag from Steam
type SuggestedTag struct {
	TagID int    `json:"tagid"`
	Name  string `json:"name"`
}

// GetUserData retrieves the user's dynamic store data including owned apps,
// wishlist, ignored apps, and suggested tags
func (s *Store) GetUserData(ctx context.Context) (*UserData, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://store.steampowered.com/dynamicstore/userdata/",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var userData UserData
	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	return &userData, nil
}
