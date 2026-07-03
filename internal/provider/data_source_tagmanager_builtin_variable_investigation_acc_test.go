package provider

import (
	"context"
	"testing"
)

// Throwaway investigation, not a real test - deleted once its output
// confirms or corrects the candidate built-in variable list this plan's
// Task 2 depends on. See docs/superpowers/plans/2026-07-03-builtin-variable-datasource.md.
func TestAccInvestigateBuiltinVariables(t *testing.T) {
	testAccPreCheck(t)
	client := testAccMatomoClient(t)

	templates, err := client.GetAvailableVariableTypes(context.Background(), "web")
	if err != nil {
		t.Fatalf("GetAvailableVariableTypes: %v", err)
	}
	t.Logf("got %d variable types from live Matomo:", len(templates))
	for _, tmpl := range templates {
		t.Logf("  id=%q name=%q category=%q", tmpl.ID, tmpl.Name, tmpl.Category)
	}
}
