package matomo

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// matomoBool decodes a boolean-shaped Matomo Tag Manager field that -
// unlike most boolean values elsewhere in this API - comes back as a JSON
// number (0 or 1) rather than a JSON boolean: confirmed against a live
// instance's real TagManager.getContainer response for
// ignoreGtmDataLayer/isTagFireLimitAllowedInPreviewMode/
// activelySyncGtmDataLayer (acceptance-test failure: "json: cannot
// unmarshal number into Go struct field Container.ignoreGtmDataLayer of
// type bool"). Also tolerates a literal JSON boolean, in case that ever
// varies by Matomo version.
type matomoBool bool

func (b *matomoBool) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*b = matomoBool(n != 0)
		return nil
	}
	var v bool
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = matomoBool(v)
	return nil
}

// Container is a Matomo Tag Manager container.
type Container struct {
	IDContainer string `json:"idcontainer"`
	IDSite      int    `json:"idsite"`
	Context     string `json:"context"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// All three confirmed against Matomo's own Dao/ContainersDao.php and
	// API.php: TINYINT(1) columns, addContainer/updateContainer parameters
	// defaulting to false at the API layer. Decoded as matomoBool, not a
	// plain bool - see its doc comment for why.
	IgnoreGtmDataLayer                 matomoBool `json:"ignoreGtmDataLayer"`
	IsTagFireLimitAllowedInPreviewMode matomoBool `json:"isTagFireLimitAllowedInPreviewMode"`
	ActivelySyncGtmDataLayer           matomoBool `json:"activelySyncGtmDataLayer"`
	// Draft is the container's mutable draft version, always present on a
	// real container (confirmed against Matomo's own TagManager source:
	// TagManager.getContainer's response nests it as draft.idcontainerversion
	// - there is no dedicated API method to fetch just the draft, and
	// getContainerVersions' entries have no boolean "isDraft" field to pick
	// it out from the version list). Unlike most other Matomo ids, this one
	// comes back as an unquoted JSON number, confirmed against a live
	// instance.
	Draft *struct {
		IDContainerVersion int `json:"idcontainerversion"`
	} `json:"draft"`
}

// AddContainer creates a new Tag Manager container and returns its ID.
func (c *Client) AddContainer(ctx context.Context, idSite int, tmContext, name, description string, ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer bool) (string, error) {
	v := url.Values{
		"idSite":                             {strconv.Itoa(idSite)},
		"context":                            {tmContext},
		"name":                               {name},
		"description":                        {description},
		"ignoreGtmDataLayer":                 {boolToIntString(ignoreGtmDataLayer)},
		"isTagFireLimitAllowedInPreviewMode": {boolToIntString(isTagFireLimitAllowedInPreviewMode)},
		"activelySyncGtmDataLayer":           {boolToIntString(activelySyncGtmDataLayer)},
	}
	var out struct {
		Value string `json:"value"`
	}
	if err := c.call(ctx, "TagManager.addContainer", v, &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

// UpdateContainer updates a container's name, description, and flags.
func (c *Client) UpdateContainer(ctx context.Context, idSite int, idContainer, name, description string, ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer bool) error {
	v := url.Values{
		"idSite":                             {strconv.Itoa(idSite)},
		"idContainer":                        {idContainer},
		"name":                               {name},
		"description":                        {description},
		"ignoreGtmDataLayer":                 {boolToIntString(ignoreGtmDataLayer)},
		"isTagFireLimitAllowedInPreviewMode": {boolToIntString(isTagFireLimitAllowedInPreviewMode)},
		"activelySyncGtmDataLayer":           {boolToIntString(activelySyncGtmDataLayer)},
	}
	return c.call(ctx, "TagManager.updateContainer", v, nil)
}

// DeleteContainer deletes a container and all its versions and releases.
func (c *Client) DeleteContainer(ctx context.Context, idSite int, idContainer string) error {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	return c.call(ctx, "TagManager.deleteContainer", v, nil)
}

// GetContainer returns a container's details.
func (c *Client) GetContainer(ctx context.Context, idSite int, idContainer string) (*Container, error) {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	var ct Container
	if err := c.call(ctx, "TagManager.getContainer", v, &ct); err != nil {
		return nil, err
	}
	return &ct, nil
}
