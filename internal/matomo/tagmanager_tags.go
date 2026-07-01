package matomo

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// Tag is a Matomo Tag Manager tag within a container version.
type Tag struct {
	IDTag           string            `json:"idtag"`
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Status          string            `json:"status"`
	Parameters      map[string]string `json:"parameters"`
	FireTriggerIDs  []string          `json:"fireTriggerIds"`
	BlockTriggerIDs []string          `json:"blockTriggerIds"`
}

// TagParams holds the fields accepted by addContainerTag/updateContainerTag.
type TagParams struct {
	Type            string
	Name            string
	Parameters      map[string]string
	FireTriggerIDs  []string
	BlockTriggerIDs []string
}

func tagParamsToValues(idSite int, idContainer, idContainerVersion string, p TagParams) (url.Values, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
	}
	params := p.Parameters
	if params == nil {
		params = map[string]string{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	v.Set("parameters", string(paramsJSON))

	fireIDs := p.FireTriggerIDs
	if fireIDs == nil {
		fireIDs = []string{}
	}
	fireJSON, err := json.Marshal(fireIDs)
	if err != nil {
		return nil, err
	}
	v.Set("fireTriggerIds", string(fireJSON))

	blockIDs := p.BlockTriggerIDs
	if blockIDs == nil {
		blockIDs = []string{}
	}
	blockJSON, err := json.Marshal(blockIDs)
	if err != nil {
		return nil, err
	}
	v.Set("blockTriggerIds", string(blockJSON))

	return v, nil
}

// AddContainerTag creates a tag in a container's version and returns its ID.
func (c *Client) AddContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion string, p TagParams) (string, error) {
	v, err := tagParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return "", err
	}
	var out struct {
		IDTag string `json:"idtag"`
	}
	if err := c.call(ctx, "TagManager.addContainerTag", v, &out); err != nil {
		return "", err
	}
	return out.IDTag, nil
}

// UpdateContainerTag updates an existing tag.
func (c *Client) UpdateContainerTag(ctx context.Context, idSite int, idContainer, idContainerVersion, idTag string, p TagParams) error {
	v, err := tagParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return err
	}
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
