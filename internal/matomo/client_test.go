package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCall_sendsPOSTWithTokenInBody confirms call() never puts token_auth
// (or any other parameter) in the URL query string - only in the POST
// body. Matomo lets a user mark their own API token as POST-only
// ("secure token"), which rejects a GET request with token_auth in the
// query string outright (confirmed against a live instance's real error:
// "Unable to authenticate with the provided token. It is either invalid,
// expired or is required to be sent as a POST parameter."). Sending
// everything via POST works unconditionally, for every token, not just
// POST-only ones.
func TestCall_sendsPOSTWithTokenInBody(t *testing.T) {
	var gotMethod string
	var gotRawQuery string
	var gotForm interface {
		Get(string) string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotRawQuery = r.URL.RawQuery
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": "ok"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "secret-token-auth-value", srv.Client())
	var out struct {
		Value string `json:"value"`
	}
	if err := client.call(context.Background(), "TagManager.getContainer", nil, &out); err != nil {
		t.Fatalf("call() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("request method = %q, want POST", gotMethod)
	}
	if strings.Contains(gotRawQuery, "secret-token-auth-value") {
		t.Errorf("token_auth leaked into URL query string: %q", gotRawQuery)
	}
	if gotRawQuery != "" {
		t.Errorf("RawQuery = %q, want empty (all params must be in the POST body)", gotRawQuery)
	}
	if got := gotForm.Get("token_auth"); got != "secret-token-auth-value" {
		t.Errorf("token_auth (from body) = %q, want secret-token-auth-value", got)
	}
	if got := gotForm.Get("method"); got != "TagManager.getContainer" {
		t.Errorf("method (from body) = %q, want TagManager.getContainer", got)
	}
}
