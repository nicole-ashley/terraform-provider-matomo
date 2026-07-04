package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
)

// Throwaway investigation, not a real test - deleted once its output settles
// whether tools/gen can auto-detect UI_CONTROL_MULTI_TUPLE parameters (e.g.
// MatomoConfiguration's customDimensions) from Matomo's discovery API, or
// whether a hand-curated override table is needed instead. See the design
// spec's section 5.5 and docs/superpowers/plans/2026-07-03-list-of-object-parameters.md.
//
// This bypasses matomo.Client entirely and hits the raw HTTP API directly,
// since matomo.Template/TemplateParam (internal/matomo/tagmanager_templates.go)
// only capture the keys this provider already knows about - any uiControl/
// uiControlAttributes keys Matomo returns would be silently dropped by the
// existing typed decode.
func TestAccInvestigateMultiTupleUIControl(t *testing.T) {
	testAccPreCheck(t)

	baseURL := os.Getenv("MATOMO_BASE_URL")
	apiToken := os.Getenv("MATOMO_API_TOKEN")

	v := url.Values{
		"module":     {"API"},
		"method":     {"TagManager.getAvailableVariableTypesInContext"},
		"idSite":     {"1"},
		"idContext":  {"web"},
		"format":     {"json"},
		"token_auth": {apiToken},
	}

	resp, err := http.Get(baseURL + "/index.php?" + v.Encode())
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}

	var groups []map[string]any
	if err := json.Unmarshal(body, &groups); err != nil {
		t.Fatalf("json.Unmarshal: %v\nraw body: %s", err, body)
	}

	for _, group := range groups {
		types, _ := group["types"].([]any)
		for _, rawType := range types {
			typeObj, ok := rawType.(map[string]any)
			if !ok {
				continue
			}
			if typeObj["id"] != "MatomoConfiguration" {
				continue
			}
			params, _ := typeObj["parameters"].([]any)
			for _, rawParam := range params {
				paramObj, ok := rawParam.(map[string]any)
				if !ok {
					continue
				}
				if paramObj["name"] != "customDimensions" {
					continue
				}
				pretty, _ := json.MarshalIndent(paramObj, "", "  ")
				t.Logf("customDimensions raw parameter object:\n%s", pretty)
			}
		}
	}
}
