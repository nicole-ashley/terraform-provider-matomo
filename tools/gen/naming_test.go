// tools/gen/naming_test.go
package main

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"CustomHtml":      "customhtml",
		"GoogleAnalytics": "googleanalytics",
		"Constant":        "constant",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamelToSnake(t *testing.T) {
	cases := map[string]string{
		"customHtml":     "custom_html",
		"htmlPosition":   "html_position",
		"fireTriggerIds": "fire_trigger_ids",
		"value":          "value",
		"URLPath":        "url_path",
	}
	for in, want := range cases {
		if got := CamelToSnake(in); got != want {
			t.Errorf("CamelToSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExportedName(t *testing.T) {
	cases := map[string]string{
		"customHtml":     "CustomHtml",
		"htmlPosition":   "HtmlPosition",
		"fireTriggerIds": "FireTriggerIds",
	}
	for in, want := range cases {
		if got := ExportedName(in); got != want {
			t.Errorf("ExportedName(%q) = %q, want %q", in, got, want)
		}
	}
}
