package matomo

import (
	"context"
	"net/url"
	"strconv"
)

// Site is a Matomo website as returned by SitesManager.getSiteFromId /
// getAllSites.
type Site struct {
	IDSite             int      `json:"idsite,string"`
	Name               string   `json:"name"`
	Timezone           string   `json:"timezone"`
	Currency           string   `json:"currency"`
	URLs               []string `json:"urls"`
	Ecommerce          bool     `json:"-"`
	ExcludedIPs        []string `json:"excluded_ips"`
	ExcludeUnknownUrls bool     `json:"-"`
	Type               string   `json:"type"`
	Group              string   `json:"group"`
}

// AddSiteParams holds the subset of SitesManager.addSite's parameters this
// provider exposes. Pointer fields are optional; nil means "let Matomo use
// its default."
type AddSiteParams struct {
	Name                string
	URLs                []string
	Timezone            *string
	Currency            *string
	Group               *string
	Type                *string
	Ecommerce           *bool
	ExcludeUnknownUrls  *bool
	ExcludedIPs         []string
}

// UpdateSiteParams mirrors AddSiteParams; all fields apply to
// SitesManager.updateSite, where a nil/zero field leaves the existing value
// unchanged.
type UpdateSiteParams struct {
	AddSiteParams
}

func (p AddSiteParams) toValues() url.Values {
	v := url.Values{}
	v.Set("siteName", p.Name)
	for _, u := range p.URLs {
		v.Add("urls[]", u)
	}
	if p.Timezone != nil {
		v.Set("timezone", *p.Timezone)
	}
	if p.Currency != nil {
		v.Set("currency", *p.Currency)
	}
	if p.Group != nil {
		v.Set("group", *p.Group)
	}
	if p.Type != nil {
		v.Set("type", *p.Type)
	}
	if p.Ecommerce != nil {
		v.Set("ecommerce", boolToIntString(*p.Ecommerce))
	}
	if p.ExcludeUnknownUrls != nil {
		v.Set("excludeUnknownUrls", boolToIntString(*p.ExcludeUnknownUrls))
	}
	for _, ip := range p.ExcludedIPs {
		v.Add("excludedIps[]", ip)
	}
	return v
}

func boolToIntString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// AddSite creates a website and returns its new site ID.
func (c *Client) AddSite(ctx context.Context, p AddSiteParams) (int, error) {
	var out struct {
		Value int `json:"value,string"`
	}
	if err := c.call(ctx, "SitesManager.addSite", p.toValues(), &out); err != nil {
		return 0, err
	}
	return out.Value, nil
}

// UpdateSite modifies an existing website.
func (c *Client) UpdateSite(ctx context.Context, idSite int, p UpdateSiteParams) error {
	v := p.toValues()
	v.Set("idSite", strconv.Itoa(idSite))
	return c.call(ctx, "SitesManager.updateSite", v, nil)
}

// DeleteSite removes a website. Does not delete its logs/archives.
func (c *Client) DeleteSite(ctx context.Context, idSite int) error {
	v := url.Values{"idSite": {strconv.Itoa(idSite)}}
	return c.call(ctx, "SitesManager.deleteSite", v, nil)
}

// GetSiteFromID retrieves a website's details by ID.
func (c *Client) GetSiteFromID(ctx context.Context, idSite int) (*Site, error) {
	v := url.Values{"idSite": {strconv.Itoa(idSite)}}
	var site Site
	if err := c.call(ctx, "SitesManager.getSiteFromId", v, &site); err != nil {
		return nil, err
	}
	return &site, nil
}

// GetAllSites returns every website (requires a superuser token).
func (c *Client) GetAllSites(ctx context.Context) ([]Site, error) {
	var sites []Site
	if err := c.call(ctx, "SitesManager.getAllSites", nil, &sites); err != nil {
		return nil, err
	}
	return sites, nil
}
