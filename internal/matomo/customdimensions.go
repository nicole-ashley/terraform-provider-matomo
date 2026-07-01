package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// DimensionExtraction is one extraction rule for a CustomDimension.
type DimensionExtraction struct {
	DimensionID int    `json:"dimension"`
	Pattern     string `json:"pattern"`
}

// CustomDimension is a Matomo custom dimension as returned by
// CustomDimensions.getConfiguredCustomDimensions. Index is the dimension's
// slot number within its Scope and is what other Matomo API calls
// (including Tag Manager's MatomoConfiguration variable) refer to it by.
type CustomDimension struct {
	ID            int                   `json:"id"`
	Name          string                `json:"name"`
	Index         int                   `json:"index,string"`
	Scope         string                `json:"scope"`
	Active        bool                  `json:"active"`
	CaseSensitive bool                  `json:"case_sensitive"`
	Extractions   []DimensionExtraction `json:"extractions"`
}

// ConfigureNewCustomDimension creates a new custom dimension in the next
// available slot for the given scope ("visit" or "action") and returns
// Matomo's internal id for it (a per-site row id, distinct from the
// dimension's Index — re-fetch via GetConfiguredCustomDimensions and match
// on this id to find which slot Matomo actually assigned).
func (c *Client) ConfigureNewCustomDimension(ctx context.Context, idSite int, name, scope string, active bool) (int, error) {
	v := url.Values{
		"idSite": {strconv.Itoa(idSite)},
		"name":   {name},
		"scope":  {scope},
		"active": {boolToIntString(active)},
	}
	var out struct {
		ID int `json:"id"`
	}
	if err := c.call(ctx, "CustomDimensions.configureNewCustomDimension", v, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// ConfigureExistingCustomDimension updates an already-configured dimension's
// name and active state. Matomo has no API to delete a custom dimension;
// setting active=false is the closest available equivalent.
func (c *Client) ConfigureExistingCustomDimension(ctx context.Context, idDimension, idSite int, name string, active bool) error {
	v := url.Values{
		"idDimension": {strconv.Itoa(idDimension)},
		"idSite":      {strconv.Itoa(idSite)},
		"name":        {name},
		"active":      {boolToIntString(active)},
	}
	return c.call(ctx, "CustomDimensions.configureExistingCustomDimension", v, nil)
}

// GetConfiguredCustomDimensions lists every custom dimension configured for
// a site, across both scopes.
func (c *Client) GetConfiguredCustomDimensions(ctx context.Context, idSite int) ([]CustomDimension, error) {
	v := url.Values{"idSite": {strconv.Itoa(idSite)}}
	var dims []CustomDimension
	if err := c.call(ctx, "CustomDimensions.getConfiguredCustomDimensions", v, &dims); err != nil {
		return nil, err
	}
	return dims, nil
}
