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
func (c *Client) call(ctx context.Context, method string, params url.Values, out interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("module", "API")
	params.Set("method", method)
	params.Set("format", "JSON")
	params.Set("token_auth", c.apiToken)

	reqURL := fmt.Sprintf("%s/index.php?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("matomo: building request for %s: %w", method, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("matomo: calling %s: %w", method, err)
	}
	defer resp.Body.Close()

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
