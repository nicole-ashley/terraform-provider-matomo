// tools/gen/spec_test.go
package main

import (
	"testing"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

func TestBuildTypeSpec(t *testing.T) {
	tmpl := matomo.Template{
		ID:          "CustomHtml",
		Name:        "Custom HTML",
		Description: "Inject custom HTML",
		Parameters: []matomo.TemplateParam{
			{Name: "customHtml", Type: "string", Description: "The HTML to inject"},
			{
				Name:            "htmlPosition",
				Type:            "string",
				Description:     "Where to inject it",
				AvailableValues: map[string]string{"top": "Top of page", "bottom": "Bottom of page"},
				Condition:       "customHtml",
			},
		},
	}

	spec, err := BuildTypeSpec("tag", tmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}
	if spec.Kind != "tag" || spec.TypeID != "CustomHtml" || spec.Slug != "customhtml" {
		t.Errorf("spec = %+v, want Kind=tag TypeID=CustomHtml Slug=customhtml", spec)
	}
	if spec.ResourceName != "matomo_tagmanager_tag_customhtml" {
		t.Errorf("spec.ResourceName = %q, want matomo_tagmanager_tag_customhtml", spec.ResourceName)
	}
	if len(spec.Params) != 2 {
		t.Fatalf("len(spec.Params) = %d, want 2", len(spec.Params))
	}

	p0 := spec.Params[0]
	if p0.MatomoName != "customHtml" || p0.TFName != "custom_html" || p0.GoFieldName != "CustomHtml" {
		t.Errorf("spec.Params[0] = %+v, want MatomoName=customHtml TFName=custom_html GoFieldName=CustomHtml", p0)
	}
	if p0.GoType != "String" {
		t.Errorf("spec.Params[0].GoType = %q, want String", p0.GoType)
	}
	if !p0.Required {
		t.Error("spec.Params[0].Required = false, want true (customHtml is required for CustomHtml)")
	}

	p1 := spec.Params[1]
	if p1.Required {
		t.Error("spec.Params[1].Required = true, want false (htmlPosition is not in requiredParams)")
	}
	if len(p1.AvailableValues) != 2 {
		t.Errorf("len(spec.Params[1].AvailableValues) = %d, want 2", len(p1.AvailableValues))
	}
	ref, ok := p1.Condition.(RefNode)
	if !ok || ref.Field != "customHtml" {
		t.Errorf("spec.Params[1].Condition = %#v, want RefNode{Field: customHtml}", p1.Condition)
	}
}

func TestBuildTypeSpec_unknownParamType(t *testing.T) {
	tmpl := matomo.Template{
		ID:         "Weird",
		Parameters: []matomo.TemplateParam{{Name: "x", Type: "float"}},
	}
	if _, err := BuildTypeSpec("tag", tmpl); err == nil {
		t.Fatal("BuildTypeSpec() error = nil, want error for unrecognized Matomo param type \"float\"")
	}
}

func TestBuildTypeSpec_unannotatedType(t *testing.T) {
	tmpl := matomo.Template{ID: "SomeBrandNewType"}
	if _, err := BuildTypeSpec("tag", tmpl); err == nil {
		t.Fatal("BuildTypeSpec() error = nil, want error for a type with no requiredParams entry")
	}
}
