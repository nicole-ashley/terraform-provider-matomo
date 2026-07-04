package main

// rowKeyNameOverrides renames a UI_CONTROL_MULTI_TUPLE row's raw Matomo
// key to a different Terraform-facing attribute name, keyed by
// "kind/typeID/matomoParamName/matomoKey". The wire key (sent to and
// received from Matomo) is always the raw Matomo name - only the
// Terraform-facing attribute name changes. Only one entry pair is known
// today: consentTypes' rows use Matomo's own "consent_type"/
// "consent_state" keys, shortened to "type"/"state" on the
// Terraform-facing side per this project's explicit naming decision (see
// docs/superpowers/plans/2026-07-03-list-of-object-parameters.md) - every
// other auto-detected MULTI_TUPLE field's raw key names read fine as-is
// and needs no entry here.
var rowKeyNameOverrides = map[string]string{
	"tag/GoogleConsentModeV2/consentTypes/consent_type":  "type",
	"tag/GoogleConsentModeV2/consentTypes/consent_state": "state",
}

// rowKeyTFName returns the Terraform-facing attribute name for one
// MULTI_TUPLE row key, applying rowKeyNameOverrides if present.
func rowKeyTFName(kind, typeID, matomoParamName, matomoKey string) string {
	lookupKey := kind + "/" + typeID + "/" + matomoParamName + "/" + matomoKey
	if override, ok := rowKeyNameOverrides[lookupKey]; ok {
		return override
	}
	return CamelToSnake(matomoKey)
}
