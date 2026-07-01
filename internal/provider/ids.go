package provider

import (
	"fmt"
	"strconv"
	"strings"
)

func buildContainerID(siteID int, idContainer string) string {
	return fmt.Sprintf("%d/%s", siteID, idContainer)
}

func parseContainerID(id string) (siteID int, idContainer string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, "", fmt.Errorf("invalid container id %q, expected format \"site_id/container_id\"", id)
	}
	siteID, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid container id %q: site_id segment is not numeric: %w", id, err)
	}
	return siteID, parts[1], nil
}

func buildDimensionID(siteID int, index int) string {
	return fmt.Sprintf("%d/%d", siteID, index)
}

func parseDimensionID(id string) (siteID int, index int, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, fmt.Errorf("invalid custom dimension id %q, expected format \"site_id/index\"", id)
	}
	siteID, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid custom dimension id %q: site_id segment is not numeric: %w", id, err)
	}
	index, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid custom dimension id %q: index segment is not numeric: %w", id, err)
	}
	return siteID, index, nil
}

func buildEntityID(siteID int, idContainer, entityID string) string {
	return fmt.Sprintf("%d/%s/%s", siteID, idContainer, entityID)
}

func parseEntityID(id string) (siteID int, idContainer, entityID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return 0, "", "", fmt.Errorf("invalid id %q, expected format \"site_id/container_id/entity_id\"", id)
	}
	siteID, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid id %q: site_id segment is not numeric: %w", id, err)
	}
	return siteID, parts[1], parts[2], nil
}
