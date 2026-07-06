// tools/gen/pluginconditional.go
package main

import "github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"

// pluginConditionalParams are real Matomo Tag Manager parameters that
// Matomo's own PHP source only adds to a type's settings when a specific
// plugin is active on the instance being queried - confirmed by reading
// each type's own source directly (grep for
// `Plugin\Manager::getInstance()->isPluginActivated(...)` guarding a
// `$this->makeSetting(...)` call). Matomo's discovery API
// (TagManager.getAvailable*TypesInContext), which tools/gen's live
// regeneration run consults, is a live reflection of whatever plugins
// happen to be active on the generating instance - so these parameters
// silently vanish from a real regeneration whenever it's run against an
// instance without that plugin active.
//
// Both known entries below (MatomoConfiguration's enableMediaAnalytics/
// enableFormAnalytics) are gated behind Matomo's MediaAnalytics/
// FormAnalytics plugins - premium, Matomo Marketplace add-ons requiring a
// paid license, which this project's own generation environment (a plain
// community-edition Docker fixture) cannot activate. Hand-added here so
// they're never silently dropped depending on which premium plugins
// happen to be licensed and active wherever tools/gen is next run for
// real - keyed by (kind, typeID), merged into that type's real discovered
// parameters in BuildTypeSpec.
//
// Description text is Matomo's own real English UI copy (confirmed
// against TagManager's lang/en.json, not paraphrased), since Matomo's
// discovery API never exposes an English string for a parameter this
// provider has to synthesize itself (only a translation key, e.g.
// "TagManager_MatomoConfigurationMatomoEnableMediaAnalyticsDescription",
// unusable directly).
var pluginConditionalParams = map[string]map[string][]matomo.TemplateParam{
	"variable": {
		"MatomoConfiguration": {
			{
				Name:        "enableFormAnalytics",
				Type:        "boolean",
				Description: "Enables the tracking of forms.",
			},
			{
				Name:        "enableMediaAnalytics",
				Type:        "boolean",
				Description: "Enables the tracking of media players.",
			},
		},
	},
}
