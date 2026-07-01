package provider

import "testing"

func TestBuildParseContainerID(t *testing.T) {
	id := buildContainerID(3, "abc123")
	if id != "3/abc123" {
		t.Fatalf("buildContainerID() = %q, want 3/abc123", id)
	}
	siteID, idContainer, err := parseContainerID(id)
	if err != nil {
		t.Fatalf("parseContainerID() error = %v", err)
	}
	if siteID != 3 || idContainer != "abc123" {
		t.Errorf("parseContainerID() = (%d, %q), want (3, abc123)", siteID, idContainer)
	}
}

func TestParseContainerID_invalid(t *testing.T) {
	cases := []string{"", "3", "3/abc/extra", "notanumber/abc123"}
	for _, c := range cases {
		if _, _, err := parseContainerID(c); err == nil {
			t.Errorf("parseContainerID(%q) error = nil, want error", c)
		}
	}
}

func TestBuildParseDimensionID(t *testing.T) {
	id := buildDimensionID(3, 1)
	if id != "3/1" {
		t.Fatalf("buildDimensionID() = %q, want 3/1", id)
	}
	siteID, index, err := parseDimensionID(id)
	if err != nil {
		t.Fatalf("parseDimensionID() error = %v", err)
	}
	if siteID != 3 || index != 1 {
		t.Errorf("parseDimensionID() = (%d, %d), want (3, 1)", siteID, index)
	}
}

func TestBuildParseEntityID(t *testing.T) {
	id := buildEntityID(3, "abc123", "5")
	if id != "3/abc123/5" {
		t.Fatalf("buildEntityID() = %q, want 3/abc123/5", id)
	}
	siteID, idContainer, entityID, err := parseEntityID(id)
	if err != nil {
		t.Fatalf("parseEntityID() error = %v", err)
	}
	if siteID != 3 || idContainer != "abc123" || entityID != "5" {
		t.Errorf("parseEntityID() = (%d, %q, %q), want (3, abc123, 5)", siteID, idContainer, entityID)
	}
}

func TestParseEntityID_invalid(t *testing.T) {
	cases := []string{"", "3/abc123", "3/abc123/5/extra"}
	for _, c := range cases {
		if _, _, _, err := parseEntityID(c); err == nil {
			t.Errorf("parseEntityID(%q) error = nil, want error", c)
		}
	}
}
