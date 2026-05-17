package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// GetSchemaItemsOptions configures a GetSchemaItems request.
type GetSchemaItemsOptions struct {
	Start    int    // pagination cursor; 0 means first page
	Language string // e.g. "en"; empty omits the param
}

// GetSchemaItems fetches item schema definitions from the
// IEconItems_{appID}/GetSchemaItems/v1 endpoint. It returns the raw response
// body as an io.ReadCloser; the caller is responsible for closing it and
// handling pagination via the "next" field in the JSON response.
func (a *API) GetSchemaItems(ctx context.Context, appID uint32, opts GetSchemaItemsOptions) (io.ReadCloser, error) {
	params, err := a.getAuthParams()
	if err != nil {
		return nil, err
	}

	if opts.Start > 0 {
		params.Set("start", strconv.Itoa(opts.Start))
	}
	if opts.Language != "" {
		params.Set("language", opts.Language)
	}

	reqURL := fmt.Sprintf("%s/IEconItems_%d/GetSchemaItems/v1/?%s", a.baseURL, appID, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, HTTPStatusError(resp.StatusCode, body)
	}

	return resp.Body, nil
}

type schemaItemsPage struct {
	Result struct {
		Status int               `json:"status"`
		Items  []json.RawMessage `json:"items"`
		Next   *int              `json:"next,omitempty"`
	} `json:"result"`
}

// GetAllSchemaItems fetches all pages of the item schema for the given app,
// returning the combined items as raw JSON. It pauses 500ms between pages to
// avoid rate limits.
func (a *API) GetAllSchemaItems(ctx context.Context, appID uint32, language string) ([]json.RawMessage, error) {
	var allItems []json.RawMessage
	opts := GetSchemaItemsOptions{Language: language}

	for {
		body, err := a.GetSchemaItems(ctx, appID, opts)
		if err != nil {
			return nil, fmt.Errorf("page start=%d: %w", opts.Start, err)
		}

		data, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		var page schemaItemsPage
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		allItems = append(allItems, page.Result.Items...)

		if page.Result.Next == nil {
			break
		}

		opts.Start = *page.Result.Next
		time.Sleep(500 * time.Millisecond)
	}

	return allItems, nil
}
