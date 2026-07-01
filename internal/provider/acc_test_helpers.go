package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// testAccPreCheck skips the calling test unless a real Matomo instance is
// configured. Every *_acc_test.go file's TestAcc* functions call this
// first, so these tests are inert (compiled, never executed) in the fast
// ci.yml job and only run in acceptance.yml.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set, skipping acceptance test")
	}
	if os.Getenv("MATOMO_BASE_URL") == "" {
		t.Skip("MATOMO_BASE_URL not set, skipping acceptance test")
	}
	if os.Getenv("MATOMO_API_TOKEN") == "" {
		t.Skip("MATOMO_API_TOKEN not set, skipping acceptance test")
	}
}

// testAccProtoV6ProviderFactories builds the real provider (not backed by
// any httptest fixture) for use in resource.Test steps. The provider reads
// MATOMO_BASE_URL/MATOMO_API_TOKEN itself via its own Configure()
// env-var fallback, so no explicit `provider "matomo" { ... }` block is
// required in test configs — an empty `provider "matomo" {}` is enough.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"matomo": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccMatomoClient builds a *matomo.Client from the same environment
// variables the provider itself reads, for use in "disappears" tests that
// need to mutate Matomo directly, bypassing Terraform.
func testAccMatomoClient(t *testing.T) *matomo.Client {
	t.Helper()
	return matomo.NewClient(os.Getenv("MATOMO_BASE_URL"), os.Getenv("MATOMO_API_TOKEN"), nil)
}
