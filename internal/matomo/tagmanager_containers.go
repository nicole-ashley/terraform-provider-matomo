package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// Container is a Matomo Tag Manager container.
type Container struct {
	IDContainer string `json:"idcontainer"`
	IDSite      int    `json:"idsite"`
	Context     string `json:"context"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Draft is the container's mutable draft version, always present on a
	// real container (confirmed against Matomo's own TagManager source:
	// TagManager.getContainer's response nests it as draft.idcontainerversion
	// - there is no dedicated API method to fetch just the draft, and
	// getContainerVersions' entries have no boolean "isDraft" field to pick
	// it out from the version list).
	Draft *struct {
		IDContainerVersion string `json:"idcontainerversion"`
	} `json:"draft"`
}

// AddContainer creates a new Tag Manager container and returns its ID.
func (c *Client) AddContainer(ctx context.Context, idSite int, tmContext, name, description string) (string, error) {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"context":     {tmContext},
		"name":        {name},
		"description": {description},
	}
	var out struct {
		Value string `json:"value"`
	}
	if err := c.call(ctx, "TagManager.addContainer", v, &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

// UpdateContainer updates a container's name and description.
func (c *Client) UpdateContainer(ctx context.Context, idSite int, idContainer, name, description string) error {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
		"name":        {name},
		"description": {description},
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
