package main

import "fmt"

// requiredParams is the hand-maintained source of truth for which
// parameters each Matomo Tag Manager type requires. Matomo's discovery
// API (TagManager.getAvailable{Tag,Trigger,Variable}TypesInContext) never
// exposes this - confirmed by reading SettingsMetadata::formatSetting()'s
// full body directly: FieldConfig::$validators (where a type's
// NotEmpty() etc. validators actually live, server-side) is never
// serialized into the response. Each entry below was added by reading
// that type's own Matomo source (grep for
// "$field->validators[] = new NotEmpty()" or similar) once.
//
// RequiredParams fails loudly for any (kind, typeID) with no entry here,
// specifically so a newly-discovered type can never silently ship with
// every parameter marked Optional by omission.
var requiredParams = map[string]map[string][]string{
	"tag": {
		"CustomHtml": {"customHtml"},
	},
	"trigger": {},
	"variable": {
		"Constant": {"value"},
	},
}

// RequiredParams returns the required parameter names for the given
// Tag Manager type. kind must be "tag", "trigger", or "variable". Returns
// an error if kind or typeID has no entry in requiredParams - callers
// must treat that as fatal (see tools/gen/main.go), never as "assume
// optional."
func RequiredParams(kind, typeID string) ([]string, error) {
	byType, ok := requiredParams[kind]
	if !ok {
		return nil, fmt.Errorf("no required-params table for kind %q (want one of tag/trigger/variable)", kind)
	}
	required, ok := byType[typeID]
	if !ok {
		return nil, fmt.Errorf("no required-params entry for %s type %q - read its Matomo source and add an entry (even an empty one) to requiredParams in tools/gen/required.go", kind, typeID)
	}
	return required, nil
}
