package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// Variable is a Matomo Tag Manager variable within a container version.
type Variable struct {
	IDVariable   string            `json:"idvariable"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Parameters   map[string]string `json:"parameters"`
	DefaultValue string            `json:"defaultValue"`
}

// VariableParams holds the fields accepted by
// addContainerVariable/updateContainerVariable.
type VariableParams struct {
	Type         string
	Name         string
	Parameters   map[string]string
	DefaultValue *string
}

func variableParamsToValues(idSite int, idContainer, idContainerVersion string, p VariableParams) url.Values {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"type":               {p.Type},
		"name":               {p.Name},
	}
	addMapParam(v, "parameters", p.Parameters)

	if p.DefaultValue != nil {
		v.Set("defaultValue", *p.DefaultValue)
	}

	return v
}

// AddContainerVariable creates a variable in a container's version and
// returns its ID.
func (c *Client) AddContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion string, p VariableParams) (string, error) {
	v := variableParamsToValues(idSite, idContainer, idContainerVersion, p)
	var out struct {
		Value int `json:"value"`
	}
	if err := c.call(ctx, "TagManager.addContainerVariable", v, &out); err != nil {
		return "", err
	}
	return strconv.Itoa(out.Value), nil
}

// UpdateContainerVariable updates an existing variable.
func (c *Client) UpdateContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string, p VariableParams) error {
	v := variableParamsToValues(idSite, idContainer, idContainerVersion, p)
	v.Set("idVariable", idVariable)
	return c.call(ctx, "TagManager.updateContainerVariable", v, nil)
}

// DeleteContainerVariable deletes a variable from a container's version.
func (c *Client) DeleteContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string) error {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idVariable":         {idVariable},
	}
	return c.call(ctx, "TagManager.deleteContainerVariable", v, nil)
}

// GetContainerVariable returns a single variable's configuration.
func (c *Client) GetContainerVariable(ctx context.Context, idSite int, idContainer, idContainerVersion, idVariable string) (*Variable, error) {
	v := url.Values{
		"idSite":             {strconv.Itoa(idSite)},
		"idContainer":        {idContainer},
		"idContainerVersion": {idContainerVersion},
		"idVariable":         {idVariable},
	}
	var variable Variable
	if err := c.call(ctx, "TagManager.getContainerVariable", v, &variable); err != nil {
		return nil, err
	}
	return &variable, nil
}
