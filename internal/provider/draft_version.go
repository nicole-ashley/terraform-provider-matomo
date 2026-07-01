package provider

import (
	"context"
	"fmt"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// resolveDraftVersionID returns the id of a container's mutable draft
// version. matomo_tagmanager_tag/_trigger/_variable resources always write
// to the draft; users never see or set a version id directly.
func resolveDraftVersionID(ctx context.Context, client *matomo.Client, siteID int, idContainer string) (string, error) {
	ct, err := client.GetContainer(ctx, siteID, idContainer)
	if err != nil {
		return "", fmt.Errorf("getting container: %w", err)
	}
	if ct.Draft == nil || ct.Draft.IDContainerVersion == "" {
		return "", fmt.Errorf("no draft version found for container %q (site %d) — every Tag Manager container should have one; this likely indicates the container was deleted out of band", idContainer, siteID)
	}
	return ct.Draft.IDContainerVersion, nil
}
