package matomo

import (
	"context"
	"net/url"
)

// TemplateParam describes one configurable parameter of a Tag Manager
// template (tag/trigger/variable type). Fields mirror
// SettingsMetadata::formatSetting()'s output (Matomo core,
// plugins/CorePluginsAdmin/SettingsMetadata.php) - confirmed against
// source, this never includes required/validator information, only
// presentation and default-value metadata. AvailableValues is nil when
// the parameter has no fixed value set.
type TemplateParam struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Description     string            `json:"description"`
	Condition       string            `json:"condition"`
	DefaultValue    any               `json:"defaultValue"`
	AvailableValues map[string]string `json:"availableValues"`
}

// Template describes one Tag Manager tag/trigger/variable type, as
// returned (after flattening the category grouping) by
// TagManager.getAvailableTagTypesInContext and its trigger/variable
// counterparts.
type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Parameters  []TemplateParam `json:"parameters"`
}

// templateCategoryGroup mirrors TemplateMetadata::formatTemplates()'s
// wire shape - confirmed against source, the API groups types by category
// rather than returning a flat list.
type templateCategoryGroup struct {
	Name  string     `json:"name"`
	Types []Template `json:"types"`
}

func (c *Client) getAvailableTemplates(ctx context.Context, method, idContext string) ([]Template, error) {
	v := url.Values{"idContext": {idContext}}
	var groups []templateCategoryGroup
	if err := c.call(ctx, method, v, &groups); err != nil {
		return nil, err
	}
	var templates []Template
	for _, g := range groups {
		templates = append(templates, g.Types...)
	}
	return templates, nil
}

// GetAvailableTagTypes returns every tag type Matomo supports in the
// given context (e.g. "web"), including third-party-plugin-contributed
// ones.
func (c *Client) GetAvailableTagTypes(ctx context.Context, idContext string) ([]Template, error) {
	return c.getAvailableTemplates(ctx, "TagManager.getAvailableTagTypesInContext", idContext)
}

// GetAvailableTriggerTypes returns every trigger type Matomo supports in
// the given context.
func (c *Client) GetAvailableTriggerTypes(ctx context.Context, idContext string) ([]Template, error) {
	return c.getAvailableTemplates(ctx, "TagManager.getAvailableTriggerTypesInContext", idContext)
}

// GetAvailableVariableTypes returns every variable type Matomo supports
// in the given context. Matomo itself filters out isPreConfigured()
// variables (the ~70 read-only built-ins) before this response is built,
// so they never appear here.
func (c *Client) GetAvailableVariableTypes(ctx context.Context, idContext string) ([]Template, error) {
	return c.getAvailableTemplates(ctx, "TagManager.getAvailableVariableTypesInContext", idContext)
}
