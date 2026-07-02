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
// Populated against matomo-org/tag-manager @ 5.x-dev. Conditionally-required
// fields (required only when a sibling field has a particular value, e.g.
// EtrackerTag's etrackerConfig when trackingType is "pageview"/"wrapper", or
// MatomoTag's idGoal/eventCategory/eventAction when trackingType is
// "goal"/"event") are deliberately left out of this table - conditional
// relationships are expressed separately via each parameter's Matomo
// `condition` string, parsed by tools/gen/condition.go into
// ConflictsWith/AlsoRequires/Eq validators (see tools/gen/spec.go,
// internal/provider/condition_validators.go). This table only records
// unconditional requiredness (an unconditional `$field->validators[] = new
// NotEmpty()`).
var requiredParams = map[string]map[string][]string{
	"tag": {
		"AddThis":                  {"AddThisPubId"},
		"Axeptio":                  {"projectId"},
		"BingUET":                  {"bingAdID"},
		"Bugsnag":                  {"apiKey"},
		"Cookiebot":                {"cookiebotId"},
		"CookieYes":                {"cookieYesWebsiteKey"},
		"CustomHtml":               {"customHtml"},
		"CustomImage":              {"customImageSrc"},
		"Drift":                    {"driftId"},
		"Emarsys":                  {"merchantId"},
		"Etracker":                 {"trackingType"},
		"FacebookPixel":            {"pixelId"},
		"GoogleAdsConversion":      {"googleAdsConversionId", "googleAdsConversionLabel"},
		"GoogleAnalytics4":         {"measurementId"},
		"GoogleAnalytics4Event":    {"eventName"},
		"GoogleAnalyticsUniversal": {"propertyId", "trackingType"},
		"GoogleConsentModeV2":      {"consentAction"},
		"GoogleTag":                {"googleTagId"},
		"Honeybadger":              {"honeybadgerApiKey"},
		"Hotjar":                   {"hjid", "hjsv"},
		"LinkedinInsight":          {"partnerId"},
		"LivezillaDynamic":         {"LivezillaDynamicID", "LivezillaDynamicDomain"},
		"Matomo":                   {"matomoConfig", "trackingType"},
		"OneTrust":                 {"domain"},
		"PingdomRUM":               {"pingdomROMId"},
		"Raygun":                   {"raygunApiKey"},
		"SentryRaven":              {"sentryDSN"},
		"Shareaholic":              {"shareaholicSiteId"},
		"TawkTo":                   {"tawkToId", "tawkToWidgetId"},
		"ThemeColor":               {"themeColor"},
		"VisualWebsiteOptimizer":   {"accountId"},
		"ZendeskChat":              {"zendeskChatId"},
	},
	"trigger": {
		"AllDownloadsClick": {"downloadExtensions"},
		"AllElementsClick":  {},
		"AllLinksClick":     {},
		"CustomEvent":       {"eventName"},
		"DomReady":          {},
		"ElementVisibility": {"selectionMethod", "fireTriggerWhen"},
		"FormSubmit":        {},
		"Fullscreen":        {"triggerAction"},
		"HistoryChange":     {},
		"JavaScriptError":   {},
		"PageView":          {},
		"ScrollReach":       {"scrollType"},
		"Timer":             {"triggerInterval"},
		"UserInteraction":   {},
		"WindowLeave":       {},
		"WindowLoaded":      {},
		"WindowUnload":      {},
	},
	"variable": {
		"ClickDataAttribute":      {"dataAttribute"},
		"ClickHtmlAttribute":      {"htmlAttribute"},
		"Constant":                {"constantValue"},
		"Cookie":                  {"cookieName"},
		"CustomJsFunction":        {"jsFunction"},
		"CustomRequestProcessing": {"jsFunction"},
		"DataLayer":               {"dataLayerName"},
		"DomElement":              {"selectionMethod"},
		"EtrackerConfiguration":   {"etrackerID"},
		"JavaScript":              {"variableName"},
		"MatomoConfiguration":     {"matomoUrl", "idSite"},
		"MetaContent":             {"metaName"},
		"ReferrerUrl":             {"urlPart"},
		"TimeSinceLoad":           {"unit"},
		"UrlParameter":            {},
		"Url":                     {"urlPart"},
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
