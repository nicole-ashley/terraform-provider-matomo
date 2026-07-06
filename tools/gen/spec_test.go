// tools/gen/spec_test.go
package main

import (
	"encoding/json"
	"testing"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// rawUIField builds the json.RawMessage form of a matomo.UIControlField
// {"key": key}, matching the real shape multiTupleRowKeys decodes -
// UIControlAttributes is map[string]json.RawMessage precisely because
// not every UIControl kind's attribute values share this shape (see
// TemplateParam's doc comment).
func rawUIField(key string) json.RawMessage {
	b, err := json.Marshal(matomo.UIControlField{Key: key})
	if err != nil {
		panic(err)
	}
	return b
}

// TestBuildTypeSpec_pluginConditionalParamsMerged confirms
// enableMediaAnalytics/enableFormAnalytics (real MatomoConfiguration
// parameters that Matomo's PHP source only adds when the premium
// MediaAnalytics/FormAnalytics plugins are active - see
// pluginConditionalParams' doc comment) are present in the built spec
// even when the discovery template used to build it - as would happen
// against an instance without those plugins active - doesn't mention
// them at all.
func TestBuildTypeSpec_pluginConditionalParamsMerged(t *testing.T) {
	tmpl := matomo.Template{
		ID:          "MatomoConfiguration",
		Name:        "Matomo Configuration",
		Description: "Configures Matomo tracking",
		Parameters: []matomo.TemplateParam{
			{Name: "matomoUrl", Type: "string", Description: "The Matomo URL"},
			{Name: "idSite", Type: "string", Description: "The idSite"},
		},
	}

	spec, err := BuildTypeSpec("variable", tmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}

	byName := make(map[string]ParamSpec, len(spec.Params))
	for _, p := range spec.Params {
		byName[p.MatomoName] = p
	}

	media, ok := byName["enableMediaAnalytics"]
	if !ok {
		t.Fatal("spec.Params missing enableMediaAnalytics")
	}
	if media.GoType != "Bool" || media.Required {
		t.Errorf("enableMediaAnalytics = %+v, want GoType=Bool Required=false", media)
	}
	if media.Description != "Enables the tracking of media players." {
		t.Errorf("enableMediaAnalytics.Description = %q, want the real Matomo UI copy", media.Description)
	}

	form, ok := byName["enableFormAnalytics"]
	if !ok {
		t.Fatal("spec.Params missing enableFormAnalytics")
	}
	if form.GoType != "Bool" || form.Required {
		t.Errorf("enableFormAnalytics = %+v, want GoType=Bool Required=false", form)
	}

	if len(spec.Params) != 4 {
		t.Errorf("len(spec.Params) = %d, want 4 (2 discovered + 2 plugin-conditional)", len(spec.Params))
	}
}

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
	if p1.AvailableValues[0] != "bottom" || p1.AvailableValues[1] != "top" {
		t.Errorf("spec.Params[1].AvailableValues = %v, want [bottom top]", p1.AvailableValues)
	}
	ref, ok := p1.Condition.(matomo.RefNode)
	if !ok || ref.Field != "custom_html" {
		t.Errorf("spec.Params[1].Condition = %#v, want matomo.RefNode{Field: custom_html} (condition field names are rewritten to TF snake_case)", p1.Condition)
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

// TestBuildTypeSpec_conditionallyRequired exercises the real Etracker
// entry in both requiredParams and conditionallyRequiredParams
// (tools/gen/required.go): trackingType is unconditionally required,
// while etrackerAddToCartProduct is only required once trackingType ==
// "addtocart" (confirmed against EtrackerTag.php - see required.go's own
// comment on conditionallyRequiredParams).
func TestBuildTypeSpec_conditionallyRequired(t *testing.T) {
	tmpl := matomo.Template{
		ID: "Etracker",
		Parameters: []matomo.TemplateParam{
			{Name: "trackingType", Type: "string", AvailableValues: map[string]string{"addtocart": "Add to cart", "pageview": "Page view"}},
			{
				Name:      "etrackerAddToCartProduct",
				Type:      "string",
				Condition: `trackingType == "addtocart"`,
			},
		},
	}

	spec, err := BuildTypeSpec("tag", tmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}

	var product ParamSpec
	found := false
	for _, p := range spec.Params {
		if p.MatomoName == "etrackerAddToCartProduct" {
			product = p
			found = true
		}
	}
	if !found {
		t.Fatal("etrackerAddToCartProduct not found in spec.Params")
	}
	if product.Required {
		t.Error("etrackerAddToCartProduct.Required = true, want false (it's only conditionally required)")
	}
	if !product.ConditionallyRequired {
		t.Error("etrackerAddToCartProduct.ConditionallyRequired = false, want true")
	}
	eq, ok := product.Condition.(matomo.EqNode)
	if !ok || eq.Field != "tracking_type" || eq.Value != "addtocart" {
		t.Errorf("etrackerAddToCartProduct.Condition = %#v, want matomo.EqNode{Field: tracking_type, Value: addtocart} (field name rewritten to TF snake_case)", product.Condition)
	}
}

// TestBuildTypeSpec_multiTupleDetection mirrors the real customDimensions
// parameter confirmed via live CI (uiControl="multituple",
// uiControlAttributes.field1.key="index", field2.key="value") - two row
// keys means a real ListOfObjects nested block, not a flat list.
func TestBuildTypeSpec_multiTupleDetection(t *testing.T) {
	tmpl := matomo.Template{
		ID: "MatomoConfiguration",
		Parameters: []matomo.TemplateParam{
			{
				Name: "customDimensions", Type: "array",
				UIControl: "multituple",
				UIControlAttributes: map[string]json.RawMessage{
					"field1": rawUIField("index"),
					"field2": rawUIField("value"),
				},
			},
		},
	}

	spec, err := BuildTypeSpec("variable", tmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}
	p := spec.Params[0]
	if !p.IsListOfObjects {
		t.Fatal("customDimensions.IsListOfObjects = false, want true")
	}
	if p.SingleKeyName != "" {
		t.Errorf("customDimensions.SingleKeyName = %q, want empty", p.SingleKeyName)
	}
	if p.BlockName != "custom_dimension" {
		t.Errorf("customDimensions.BlockName = %q, want custom_dimension", p.BlockName)
	}
	if len(p.RowKeys) != 2 {
		t.Fatalf("len(customDimensions.RowKeys) = %d, want 2", len(p.RowKeys))
	}
	if p.RowKeys[0] != (RowKeySpec{MatomoKey: "index", TFName: "index", GoFieldName: "Index"}) {
		t.Errorf("customDimensions.RowKeys[0] = %+v, want {index index Index}", p.RowKeys[0])
	}
	if p.RowKeys[1] != (RowKeySpec{MatomoKey: "value", TFName: "value", GoFieldName: "Value"}) {
		t.Errorf("customDimensions.RowKeys[1] = %+v, want {value value Value}", p.RowKeys[1])
	}
}

// TestBuildTypeSpec_singleKeyMultiTuple mirrors the real domains
// parameter: also UI_CONTROL_MULTI_TUPLE, but with only one row key - it
// stays a flat List (no nested block), only its wire encoding differs.
func TestBuildTypeSpec_singleKeyMultiTuple(t *testing.T) {
	tmpl := matomo.Template{
		ID: "MatomoConfiguration",
		Parameters: []matomo.TemplateParam{
			{
				Name: "domains", Type: "array",
				UIControl: "multituple",
				UIControlAttributes: map[string]json.RawMessage{
					"field1": rawUIField("domain"),
				},
			},
		},
	}

	spec, err := BuildTypeSpec("variable", tmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}
	p := spec.Params[0]
	if p.IsListOfObjects {
		t.Error("domains.IsListOfObjects = true, want false")
	}
	if p.GoType != "List" {
		t.Errorf("domains.GoType = %q, want List", p.GoType)
	}
	if p.SingleKeyName != "domain" {
		t.Errorf("domains.SingleKeyName = %q, want domain", p.SingleKeyName)
	}
}

// TestBuildTypeSpec_consentTypesKeyOverride mirrors the real consentTypes
// parameter, confirming rowKeyNameOverrides renames its raw
// consent_type/consent_state wire keys to the shorter type/state
// Terraform-facing names per this project's naming decision, without
// changing the wire key used in RowKeySpec.MatomoKey.
func TestBuildTypeSpec_consentTypesKeyOverride(t *testing.T) {
	tmpl := matomo.Template{
		ID: "GoogleConsentModeV2",
		Parameters: []matomo.TemplateParam{
			{
				Name: "consentTypes", Type: "array",
				UIControl: "multituple",
				UIControlAttributes: map[string]json.RawMessage{
					"field1": rawUIField("consent_type"),
					"field2": rawUIField("consent_state"),
				},
			},
		},
	}

	spec, err := BuildTypeSpec("tag", tmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}
	p := spec.Params[0]
	if p.RowKeys[0] != (RowKeySpec{MatomoKey: "consent_type", TFName: "type", GoFieldName: "Type"}) {
		t.Errorf("consentTypes.RowKeys[0] = %+v, want {consent_type type Type}", p.RowKeys[0])
	}
	if p.RowKeys[1] != (RowKeySpec{MatomoKey: "consent_state", TFName: "state", GoFieldName: "State"}) {
		t.Errorf("consentTypes.RowKeys[1] = %+v, want {consent_state state State}", p.RowKeys[1])
	}
	// consentTypes is the one field with a listOfObjectsAsAttributeOverrides
	// entry - Matomo defaults it server-side to 7 non-empty rows, which a
	// schema.ListNestedBlock can never represent (confirmed against a real
	// acceptance-test failure - see spec.go's doc comment on the override
	// table). Must render as a Computed ListNestedAttribute instead.
	if !p.AsAttribute {
		t.Error("consentTypes.AsAttribute = false, want true (listOfObjectsAsAttributeOverrides entry)")
	}
}

// TestBuildTypeSpec_nonMultiTupleUIControlAttributesShape is a regression
// test for a real failure hit against live CI: some other UIControl kind
// (confirmed to be UI_CONTROL_SINGLE_SELECT, a different, unrelated
// presentation hint per the design spec's section 4) has at least one
// uiControlAttributes value that is a plain JSON string, not an object
// with a "key" field - decoding every parameter's uiControlAttributes
// eagerly as map[string]matomo.UIControlField broke discovery for every
// type at once with "cannot unmarshal string into ... UIControlField".
// BuildTypeSpec must never touch UIControlAttributes at all unless
// UIControl == "multituple" first.
func TestBuildTypeSpec_nonMultiTupleUIControlAttributesShape(t *testing.T) {
	tmpl := matomo.Template{
		ID: "GoogleConsentModeV2",
		Parameters: []matomo.TemplateParam{
			{
				Name: "consentAction", Type: "string",
				UIControl: "singleselect",
				UIControlAttributes: map[string]json.RawMessage{
					"formats": json.RawMessage(`"some plain string value"`),
				},
			},
		},
	}

	spec, err := BuildTypeSpec("tag", tmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}
	p := spec.Params[0]
	if p.IsListOfObjects || p.SingleKeyName != "" {
		t.Errorf("consentAction (UIControl=singleselect, GoType=String) should never reach multiTupleRowKeys - got IsListOfObjects=%v SingleKeyName=%q", p.IsListOfObjects, p.SingleKeyName)
	}
}

func TestBuildTypeSpec_conditionallyRequiredWithoutCondition(t *testing.T) {
	// A parameter listed in conditionallyRequiredParams but with no
	// `condition` string from Matomo's API is a data-entry mistake in
	// required.go - BuildTypeSpec must fail loudly rather than silently
	// treat it as unconditionally optional.
	tmpl := matomo.Template{
		ID: "Etracker",
		Parameters: []matomo.TemplateParam{
			{Name: "trackingType", Type: "string"},
			{Name: "etrackerAddToCartProduct", Type: "string"}, // no Condition
		},
	}
	if _, err := BuildTypeSpec("tag", tmpl); err == nil {
		t.Fatal("BuildTypeSpec() error = nil, want error for a conditionallyRequiredParams entry with no condition")
	}
}

// TestBuildTypeSpec_trimTrailingNewlineOverride confirms the two known
// free-form code fields (reported to report a perpetual plan diff when
// configured via HCL heredoc) get TrimTrailingNewline set, and that an
// unrelated String parameter on the same types does not.
func TestBuildTypeSpec_trimTrailingNewlineOverride(t *testing.T) {
	jsTmpl := matomo.Template{
		ID: "CustomJsFunction",
		Parameters: []matomo.TemplateParam{
			{Name: "jsFunction", Type: "string"},
		},
	}
	spec, err := BuildTypeSpec("variable", jsTmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}
	if !spec.Params[0].TrimTrailingNewline {
		t.Error("CustomJsFunction.jsFunction.TrimTrailingNewline = false, want true")
	}

	htmlTmpl := matomo.Template{
		ID: "CustomHtml",
		Parameters: []matomo.TemplateParam{
			{Name: "customHtml", Type: "string"},
			{Name: "htmlPosition", Type: "string", Condition: "customHtml"},
		},
	}
	spec, err = BuildTypeSpec("tag", htmlTmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}
	byName := make(map[string]ParamSpec, len(spec.Params))
	for _, p := range spec.Params {
		byName[p.MatomoName] = p
	}
	if !byName["customHtml"].TrimTrailingNewline {
		t.Error("CustomHtml.customHtml.TrimTrailingNewline = false, want true")
	}
	if byName["htmlPosition"].TrimTrailingNewline {
		t.Error("CustomHtml.htmlPosition.TrimTrailingNewline = true, want false (not in the override table)")
	}
}
