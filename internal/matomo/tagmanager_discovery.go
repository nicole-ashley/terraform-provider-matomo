package matomo

import "context"

// Context describes one Tag Manager context (e.g. "web", "amp", "mobile"),
// as returned by TagManager.getAvailableContexts. Only contexts with at
// least one available tag type are included (confirmed against
// matomo-org/tag-manager's API.php: getAvailableContexts() filters out
// any context whose getAvailableTagTypesInContext() call returns empty).
type Context struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetAvailableContexts returns every context this Matomo instance
// supports.
func (c *Client) GetAvailableContexts(ctx context.Context) ([]Context, error) {
	var contexts []Context
	if err := c.call(ctx, "TagManager.getAvailableContexts", nil, &contexts); err != nil {
		return nil, err
	}
	return contexts, nil
}

// Environment describes one Tag Manager publish environment (e.g.
// "live", "dev", "staging"), as returned by
// TagManager.getAvailableEnvironments.
type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetAvailableEnvironments returns every environment configured on this
// Matomo instance.
func (c *Client) GetAvailableEnvironments(ctx context.Context) ([]Environment, error) {
	var environments []Environment
	if err := c.call(ctx, "TagManager.getAvailableEnvironments", nil, &environments); err != nil {
		return nil, err
	}
	return environments, nil
}
