// tools/gen/emit_test.go
package main

import (
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

func TestRenderSchema_parsesAsValidGo(t *testing.T) {
	spec := TypeSpec{
		Kind:         "tag",
		TypeID:       "CustomHtml",
		Slug:         "customhtml",
		ResourceName: "matomo_tagmanager_tag_customhtml",
		Description:  "Inject custom HTML",
		Params: []ParamSpec{
			{MatomoName: "customHtml", TFName: "custom_html", GoFieldName: "CustomHtml", Description: "The HTML to inject", GoType: "String", Required: true},
			{
				MatomoName: "htmlPosition", TFName: "html_position", GoFieldName: "HtmlPosition",
				Description: "Where to inject it", GoType: "String", Required: false,
				AvailableValues: []string{"top", "bottom"},
				Condition:       matomo.RefNode{Field: "custom_html"},
			},
		},
	}

	src, err := RenderSchema(spec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "tag_customhtml.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as valid Go: %v\n---\n%s", err, src)
	}

	got := string(src)
	for _, want := range []string{
		"tagCustomhtmlModel",
		`"custom_html"`,
		"Required: true",
		`TypeID:       "CustomHtml"`,
		`ResourceName: "matomo_tagmanager_tag_customhtml"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q; full source:\n%s", want, got)
		}
	}

	// gofmt column-aligns struct field declarations, so the amount of
	// whitespace between the field name and its type is not fixed --
	// match loosely with a regexp instead of a literal substring.
	for _, wantPattern := range []string{
		`CustomHtml\s+types\.String`,
		`HtmlPosition\s+types\.String`,
	} {
		if !regexp.MustCompile(wantPattern).MatchString(got) {
			t.Errorf("generated source missing pattern %q; full source:\n%s", wantPattern, got)
		}
	}
}

// TestRenderSchema_conditionallyRequired mirrors the real Etracker case
// end to end: a param with ConditionallyRequired set must render a
// conditionRequiredValidator wired to its (already TF-snake_case)
// condition, with the internal/matomo package imported to support it -
// and the result must still be valid, compilable Go.
func TestRenderSchema_conditionallyRequired(t *testing.T) {
	spec := TypeSpec{
		Kind:         "tag",
		TypeID:       "Etracker",
		Slug:         "etracker",
		ResourceName: "matomo_tagmanager_tag_etracker",
		Description:  "eTracker Analytics",
		Params: []ParamSpec{
			{
				MatomoName: "trackingType", TFName: "tracking_type", GoFieldName: "TrackingType",
				GoType: "String", Required: true,
				AvailableValues: []string{"addtocart", "pageview"},
			},
			{
				MatomoName: "etrackerAddToCartProduct", TFName: "etracker_add_to_cart_product", GoFieldName: "EtrackerAddToCartProduct",
				GoType: "String", Required: false, ConditionallyRequired: true,
				Condition: matomo.EqNode{Field: "tracking_type", Value: "addtocart"},
			},
		},
	}

	src, err := RenderSchema(spec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "tag_etracker.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as valid Go: %v\n---\n%s", err, src)
	}

	got := string(src)
	for _, want := range []string{
		`"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"`,
		`conditionRequiredValidator{Condition: matomo.EqNode{Field: "tracking_type", Value: "addtocart", Negate: false}}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q; full source:\n%s", want, got)
		}
	}
}
