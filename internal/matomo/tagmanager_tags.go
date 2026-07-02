package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// Tag is a Matomo Tag Manager tag within a container version.
type Tag struct {
	IDTag      int       `json:"idtag"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Parameters ParamsMap `json:"parameters"`
	// Confirmed against Matomo's own TagTest.php fixture: the response keys
	// are fire_trigger_ids/block_trigger_ids (snake_case), unlike the
	// fireTriggerIds/blockTriggerIds (camelCase) request parameters used to
	// set them. Confirmed against a live instance: unlike most other
	// Matomo ids, these array elements are unquoted JSON numbers, not
	// strings.
	FireTriggerIDs  []int `json:"fire_trigger_ids"`
	BlockTriggerIDs []int `json:"block_trigger_ids"`
}

// TagParams holds the fields accepted by addContainerTag/updateContainerTag.
type TagParams struct {
	Type            string
	Name            string
	Parameters      ParamsMap
	FireTriggerIDs  []string
	BlockTriggerIDs []string
}

func tagParamsToValues(idSite int, idContainer, idContainerVersion string, p TagParams) url.Values {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
	}
	addParamsMap(v, "parameters", p.Parameters)
	addArrayParam(v, "fireTriggerIds", p.FireTriggerIDs)
	addArrayParam(v, "blockTriggerIds", p.BlockTriggerIDs)

	return v
}

// AddContainerTag creates a tag in a container's version and returns its ID.
func (c *Client) AddContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion string, p TagParams) (string, error) {
	v := tagParamsToValues(idSite, idContainer, idContainerVersion, p)
	var out struct {
		Value int `json:"value"`
	}
	if err := c.call(ctx, "TagManager.addContainerTag", v, &out); err != nil {
		return "", err
	}
	return strconv.Itoa(out.Value), nil
}

// UpdateContainerTag updates an existing tag.
func (c *Client) UpdateContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string, p TagParams) error {
	v := tagParamsToValues(idSite, idContainer, idContainerVersion, p)
	v.Set("idTag", idTag)
	return c.call(ctx, "TagManager.updateContainerTag", v, nil)
}

// DeleteContainerTag deletes a tag from a container's version.
func (c *Client) DeleteContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTag":              {idTag},
	}
	return c.call(ctx, "TagManager.deleteContainerTag", v, nil)
}

// GetContainerTag returns a single tag's configuration.
func (c *Client) GetContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) (*Tag, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTag":              {idTag},
	}
	var tag Tag
	if err := c.call(ctx, "TagManager.getContainerTag", v, &tag); err != nil {
		return nil, err
	}
	return &tag, nil
}

func tagIdentityValues(idSite int, idContainer, idContainerVersion, idTag string) url.Values {
	return url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTag":              {idTag},
	}
}

// PauseContainerTag pauses a tag in the draft version. This does not take
// effect on a live (published) container until a new version is created
// and published.
func (c *Client) PauseContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error {
	return c.call(ctx, "TagManager.pauseContainerTag", tagIdentityValues(idSite, idContainer, idContainerVersion, idTag), nil)
}

// ResumeContainerTag resumes a paused tag in the draft version. See
// PauseContainerTag's note on when this takes effect live.
func (c *Client) ResumeContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string) error {
	return c.call(ctx, "TagManager.resumeContainerTag", tagIdentityValues(idSite, idContainer, idContainerVersion, idTag), nil)
}
