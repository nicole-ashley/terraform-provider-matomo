package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// CreateContainerVersion snapshots a container's current draft into a
// new, named, immutable version and returns its id. name must be 1-50
// characters (a Matomo-side constraint - not re-validated here).
func (c *Client) CreateContainerVersion(ctx context.Context, idSite int, idContainer, name, description string) (int, error) {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
		"name":        {name},
		"description": {description},
	}
	var out struct {
		Value int `json:"value"`
	}
	if err := c.call(ctx, "TagManager.createContainerVersion", v, &out); err != nil {
		return 0, err
	}
	return out.Value, nil
}

// PublishContainerVersion publishes an existing container version
// snapshot to the given environment. idContainerVersion must refer to a
// version already created via CreateContainerVersion - Matomo does not
// allow publishing the mutable draft directly.
func (c *Client) PublishContainerVersion(ctx context.Context, idSite int, idContainer string, idContainerVersion int, environment string) error {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {strconv.Itoa(idContainerVersion)},
		"environment":        {environment},
	}
	return c.call(ctx, "TagManager.publishContainerVersion", v, nil)
}

// EnablePreviewMode turns on preview mode for a container's current
// draft version.
func (c *Client) EnablePreviewMode(ctx context.Context, idSite int, idContainer string) error {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	return c.call(ctx, "TagManager.enablePreviewMode", v, nil)
}

// DisablePreviewMode turns off preview mode for a container's current
// draft version.
func (c *Client) DisablePreviewMode(ctx context.Context, idSite int, idContainer string) error {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	return c.call(ctx, "TagManager.disablePreviewMode", v, nil)
}
