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
	versions, err := client.GetContainerVersions(ctx, siteID, idContainer)
	if err != nil {
		return "", fmt.Errorf("listing container versions: %w", err)
	}
	for _, v := range versions {
		if v.IsDraft {
			return v.IDContainerVersion, nil
		}
	}
	return "", fmt.Errorf("no draft version found for container %q (site %d) — every Tag Manager container should have one; this likely indicates the container was deleted out of band", idContainer, siteID)
}
