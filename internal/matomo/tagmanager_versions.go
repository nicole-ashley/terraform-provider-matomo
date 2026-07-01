package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// ContainerVersion is a Tag Manager container version.
type ContainerVersion struct {
	IDContainerVersion string `json:"idcontainerversion"`
	Name               string `json:"name"`
	IsDraft            bool   `json:"isDraft"`
}

// GetContainerVersions lists every version of a container, including the
// mutable draft (IsDraft == true).
func (c *Client) GetContainerVersions(ctx context.Context, idSite int, idContainer string) ([]ContainerVersion, error) {
	v := url.Values{
		"idSite":      {strconv.Itoa(idSite)},
		"idContainer": {idContainer},
	}
	var versions []ContainerVersion
	if err := c.call(ctx, "TagManager.getContainerVersions", v, &versions); err != nil {
		return nil, err
	}
	return versions, nil
}
