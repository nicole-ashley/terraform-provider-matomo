package matomo

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// Condition is one trigger-firing condition.
type Condition struct {
	Comparison            string `json:"comparison"`
	ActualValueVariableID string `json:"actual"`
	ExpectedValue         string `json:"value"`
}

// Trigger is a Matomo Tag Manager trigger within a container version.
type Trigger struct {
	IDTrigger  string            `json:"idtrigger"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Parameters map[string]string `json:"parameters"`
	Conditions []Condition       `json:"conditions"`
}

// TriggerParams holds the fields accepted by
// addContainerTrigger/updateContainerTrigger.
type TriggerParams struct {
	Type       string
	Name       string
	Parameters map[string]string
	Conditions []Condition
}

func triggerParamsToValues(idSite int, idContainer, idContainerVersion string, p TriggerParams) (url.Values, error) {
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

	conditions := p.Conditions
	if conditions == nil {
		conditions = []Condition{}
	}
	conditionsJSON, err := json.Marshal(conditions)
	if err != nil {
		return nil, err
	}
	v.Set("conditions", string(conditionsJSON))

	return v, nil
}

// AddContainerTrigger creates a trigger in a container's version and
// returns its ID.
func (c *Client) AddContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion string, p TriggerParams) (string, error) {
	v, err := triggerParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return "", err
	}
	var out struct {
		IDTrigger string `json:"idtrigger"`
	}
	if err := c.call(ctx, "TagManager.addContainerTrigger", v, &out); err != nil {
		return "", err
	}
	return out.IDTrigger, nil
}

// UpdateContainerTrigger updates an existing trigger.
func (c *Client) UpdateContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string, p TriggerParams) error {
	v, err := triggerParamsToValues(idSite, idContainer, idContainerVersion, p)
	if err != nil {
		return err
	}
	v.Set("idTrigger", idTrigger)
	return c.call(ctx, "TagManager.updateContainerTrigger", v, nil)
}

// DeleteContainerTrigger deletes a trigger from a container's version.
func (c *Client) DeleteContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string) error {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTrigger":          {idTrigger},
	}
	return c.call(ctx, "TagManager.deleteContainerTrigger", v, nil)
}

// GetContainerTrigger returns a single trigger's configuration.
func (c *Client) GetContainerTrigger(ctx context.Context, idSite int, idContainer, idContainerVersion, idTrigger string) (*Trigger, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idTrigger":          {idTrigger},
	}
	var trig Trigger
	if err := c.call(ctx, "TagManager.getContainerTrigger", v, &trig); err != nil {
		return nil, err
	}
	return &trig, nil
}
