package matomo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client is a thin, typed wrapper over the Matomo HTTP API
// (module=API&method=...&format=JSON). It is not a generic RPC proxy:
// each Matomo API method gets its own typed Go method elsewhere in this
// package, built on top of call.
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// NewClient creates a Matomo API client. baseURL is the Matomo instance's
// root URL (e.g. "https://analytics.example.com"), without a trailing
// slash or "/index.php" suffix.
func NewClient(baseURL, apiToken string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiToken:   apiToken,
		httpClient: httpClient,
	}
}

// APIError represents Matomo's {"result":"error","message":"..."} envelope.
type APIError struct {
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

type errorEnvelope struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

// call invokes a Matomo API method and decodes the JSON response into out.
// params must not set "module", "method", "format", or "token_auth" — call
// sets those itself.
//
// Sent as a POST with every parameter (including token_auth) in the
// request body, never in the URL query string. Matomo lets a user mark
// their own API token as POST-only ("secure token", added to prevent a
// token leaking via server/proxy access logs or a browser's URL/referrer
// history) - a GET request with token_auth in the query string is
// rejected outright for such a token with "Unable to authenticate with
// the provided token. It is either invalid, expired or is required to be
// sent as a POST parameter." (Matomo's own real error message, confirmed
// against a live instance whose token was configured this way). Matomo's
// reporting API accepts POST for every method, read or write, so this is
// safe unconditionally - not just a workaround for the POST-only case.
func (c *Client) call(ctx context.Context, method string, params url.Values, out interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("module", "API")
	params.Set("method", method)
	params.Set("format", "JSON")
	params.Set("token_auth", c.apiToken)

	reqURL := fmt.Sprintf("%s/index.php", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("matomo: building request for %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("matomo: calling %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("matomo: reading response for %s: %w", method, err)
	}

	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Result == "error" {
		return &APIError{Message: env.Message}
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("matomo: decoding response for %s: %w", method, err)
	}
	return nil
}
