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

func TestRenderSchema_commonDescriptionAndPriority(t *testing.T) {
	tagSpec := TypeSpec{
		Kind:         "tag",
		TypeID:       "CustomHtml",
		Slug:         "customhtml",
		ResourceName: "matomo_tagmanager_tag_customhtml",
		Description:  "Inject custom HTML",
		Params: []ParamSpec{
			{MatomoName: "customHtml", TFName: "custom_html", GoFieldName: "CustomHtml", GoType: "String", Required: true},
		},
	}
	src, err := RenderSchema(tagSpec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}
	got := string(src)
	for _, want := range []string{
		`"description": schema.StringAttribute{`,
		`"priority": schema.Int64Attribute{`,
		`int64planmodifier.UseStateForUnknown()`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tag schema missing %q; full source:\n%s", want, got)
		}
	}

	triggerSpec := TypeSpec{
		Kind:         "trigger",
		TypeID:       "PageView",
		Slug:         "pageview",
		ResourceName: "matomo_tagmanager_trigger_pageview",
		Description:  "Triggered on page view",
	}
	src, err = RenderSchema(triggerSpec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}
	got = string(src)
	if !strings.Contains(got, `"description": schema.StringAttribute{`) {
		t.Errorf("trigger schema missing description attribute; full source:\n%s", got)
	}
	if strings.Contains(got, `"priority"`) {
		t.Errorf("trigger schema must not have a priority attribute; full source:\n%s", got)
	}

	variableSpec := TypeSpec{
		Kind:         "variable",
		TypeID:       "Constant",
		Slug:         "constant",
		ResourceName: "matomo_tagmanager_variable_constant",
		Description:  "A constant value",
	}
	src, err = RenderSchema(variableSpec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}
	got = string(src)
	if !strings.Contains(got, `"description": schema.StringAttribute{`) {
		t.Errorf("variable schema missing description attribute; full source:\n%s", got)
	}
	if strings.Contains(got, `"priority"`) {
		t.Errorf("variable schema must not have a priority attribute; full source:\n%s", got)
	}
}

// TestRenderSchema_optionalFieldsAreComputed exercises every Go type's
// Optional-field code path (String/Bool/Int64/Float64/List) to prove each
// renders valid, compiling Go: String/Bool/Int64/Float64 get a Computed +
// UseStateForUnknown attribute (with the matching plan modifier import) -
// the fix for a live-acceptance-test failure where a Matomo-defaulted
// value for an unset Optional field (e.g. a boolean parameter coming
// back false rather than absent) caused a perpetual "refresh plan not
// empty" diff against a bare Optional attribute. List is deliberately
// NOT made Computed - its generated Go field type ([]types.String) can't
// represent an unknown list, and a live acceptance-test run confirmed
// that combination fails outright ("Value Conversion Error ... Received
// unknown value").
func TestRenderSchema_optionalFieldsAreComputed(t *testing.T) {
	spec := TypeSpec{
		Kind:         "trigger",
		TypeID:       "Everything",
		Slug:         "everything",
		ResourceName: "matomo_tagmanager_trigger_everything",
		Description:  "exercises every optional Go type",
		Params: []ParamSpec{
			{MatomoName: "s", TFName: "s", GoFieldName: "S", GoType: "String", Required: false},
			{MatomoName: "b", TFName: "b", GoFieldName: "B", GoType: "Bool", Required: false},
			{MatomoName: "i", TFName: "i", GoFieldName: "I", GoType: "Int64", Required: false},
			{MatomoName: "f", TFName: "f", GoFieldName: "F", GoType: "Float64", Required: false},
			{MatomoName: "l", TFName: "l", GoFieldName: "L", GoType: "List", Required: false},
		},
	}

	src, err := RenderSchema(spec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "trigger_everything.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as valid Go: %v\n---\n%s", err, src)
	}

	got := string(src)
	for _, want := range []string{
		`"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"`,
		`"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"`,
		`"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"`,
		`PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}`,
		`PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}`,
		`PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}`,
		`PlanModifiers: []planmodifier.Float64{float64planmodifier.UseStateForUnknown()}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q; full source:\n%s", want, got)
		}
	}
	if strings.Contains(got, "listplanmodifier") {
		t.Errorf("generated source references listplanmodifier, want none (List params are not Computed): full source:\n%s", got)
	}
	// The "l" field itself must still be Optional (not Computed) - only
	// check its own attribute block, not "L []types.String" the struct
	// field declaration.
	if regexp.MustCompile(`"l":\s*schema\.ListAttribute\{[^}]*Computed`).MatchString(got) {
		t.Errorf("generated source marks the List param \"l\" Computed, want Optional only: full source:\n%s", got)
	}
}

// TestRenderSchema_listParamsUseNativeArrayEncoding proves ToParams/
// FromParams wire a List-typed parameter through matomo.ListParam/
// ParamValue.List, not a delimiter-joined string - a live acceptance
// test only ever exercised a single-element list, which can't tell a
// correct array encoding apart from the lossy comma-joined-string
// encoding it replaced (a multi-element value, or any element
// containing a comma, would have round-tripped corrupted, and Matomo's
// dispatcher rejects a joined string for a genuinely array-typed field
// outright in the first place).
func TestRenderSchema_listParamsUseNativeArrayEncoding(t *testing.T) {
	spec := TypeSpec{
		Kind:         "tag",
		TypeID:       "ListExample",
		Slug:         "listexample",
		ResourceName: "matomo_tagmanager_tag_listexample",
		Description:  "exercises List param wire encoding",
		Params: []ParamSpec{
			{MatomoName: "requiredList", TFName: "required_list", GoFieldName: "RequiredList", GoType: "List", Required: true},
			{MatomoName: "optionalList", TFName: "optional_list", GoFieldName: "OptionalList", GoType: "List", Required: false},
		},
	}

	src, err := RenderSchema(spec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "tag_listexample.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as valid Go: %v\n---\n%s", err, src)
	}

	got := string(src)
	for _, want := range []string{
		`func (m *tagListexampleModel) ToParams() matomo.ParamsMap {`,
		`p["requiredList"] = matomo.ListParam(stringSliceFromModel(m.RequiredList))`,
		`p["optionalList"] = matomo.ListParam(stringSliceFromModel(m.OptionalList))`,
		`func (m *tagListexampleModel) FromParams(p matomo.ParamsMap) {`,
		`m.RequiredList = paramListValue(p["requiredList"].List)`,
		`m.OptionalList = paramListValue(v.List)`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q; full source:\n%s", want, got)
		}
	}
	if strings.Contains(got, "paramListString") || strings.Contains(got, `strings.Join`) {
		t.Errorf("generated source still comma-joins a List param; full source:\n%s", got)
	}
}

// TestRenderSchema_listOfObjectsAndSingleKeyList exercises both auto-detected
// MULTI_TUPLE shapes end-to-end through the real emitter and template - a
// two-key ListOfObjects parameter (mirroring the real customDimensions,
// confirmed generically exposed via live CI) and a one-key SingleKeyName
// parameter (mirroring the real domains) on the same synthetic type. Since
// RenderSchema gofmt's its output, a template bug that produces invalid Go
// (mismatched braces, a bad field reference, etc.) fails this test even
// without a live Matomo instance.
func TestRenderSchema_listOfObjectsAndSingleKeyList(t *testing.T) {
	spec := TypeSpec{
		Kind:         "variable",
		TypeID:       "MultiTupleExample",
		Slug:         "multituplexample",
		ResourceName: "matomo_tagmanager_variable_multituplexample",
		Description:  "exercises ListOfObjects and single-key List param wire encoding",
		Params: []ParamSpec{
			{
				MatomoName: "customDimensions", TFName: "custom_dimensions", GoFieldName: "CustomDimensions",
				Description: "Custom dimensions", GoType: "List", Required: false,
				IsListOfObjects: true,
				BlockName:       "custom_dimension",
				RowKeys: []RowKeySpec{
					{MatomoKey: "index", TFName: "index", GoFieldName: "Index"},
					{MatomoKey: "value", TFName: "value", GoFieldName: "Value"},
				},
			},
			{
				MatomoName: "domains", TFName: "domains", GoFieldName: "Domains",
				Description: "Domains", GoType: "List", Required: false,
				SingleKeyName: "domain",
			},
		},
	}

	src, err := RenderSchema(spec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "variable_multituplexample.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as valid Go: %v\n---\n%s", err, src)
	}

	got := string(src)
	for _, want := range []string{
		"type variableMultituplexampleCustomDimensionModel struct {",
		`"custom_dimension": schema.ListNestedBlock{`,
		`"index": schema.StringAttribute{`,
		`"value": schema.StringAttribute{`,
		"rows := make([]map[string]string, len(m.CustomDimensions))",
		"rows[i] = map[string]string{",
		`p["customDimensions"] = matomo.ListOfObjectsParam(rows)`,
		`p["domains"] = matomo.WrapSingleKeyParam("domain", stringSliceFromModel(m.Domains))`,
		"m.CustomDimensions = make([]variableMultituplexampleCustomDimensionModel, len(v.ListOfObjects))",
		"vals := make([]string, len(v.ListOfObjects))",
		`vals[i] = row["domain"]`,
		"m.Domains = paramListValue(vals)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q; full source:\n%s", want, got)
		}
	}

	// gofmt column-aligns struct field declarations and map-literal
	// key/value pairs, so the amount of whitespace varies with sibling
	// widths - match loosely with regexps instead of literal substrings
	// (same reasoning as TestRenderSchema_parsesAsValidGo above).
	for _, wantPattern := range []string{
		`CustomDimensions\s+\[\]variableMultituplexampleCustomDimensionModel\s+` + "`tfsdk:\"custom_dimension\"`",
		`Domains\s+\[\]types\.String\s+` + "`tfsdk:\"domains\"`",
		`"index":\s+row\.Index\.ValueString\(\),`,
		`"value":\s+row\.Value\.ValueString\(\),`,
		`Index:\s+types\.StringValue\(row\["index"\]\),`,
		`Value:\s+types\.StringValue\(row\["value"\]\),`,
	} {
		if !regexp.MustCompile(wantPattern).MatchString(got) {
			t.Errorf("generated source missing pattern %q; full source:\n%s", wantPattern, got)
		}
	}

	if strings.Contains(got, "ListParam(stringSliceFromModel(m.Domains))") {
		t.Errorf("domains still uses plain ListParam instead of WrapSingleKeyParam; full source:\n%s", got)
	}
	if strings.Contains(got, `"custom_dimensions": schema.ListAttribute{`) {
		t.Errorf("customDimensions should not appear as a flat Attributes entry (it moved to Blocks); full source:\n%s", got)
	}
}

// TestRenderSchema_listOfObjectsAsAttribute exercises the AsAttribute
// override (mirrors the real consentTypes/consent_type field) end-to-end:
// a ListOfObjects parameter with AsAttribute=true must render as a
// Computed schema.ListNestedAttribute inside Attributes, NOT as a
// schema.ListNestedBlock inside Blocks - the whole reason this shape
// exists is that Matomo defines a non-empty server-side default for this
// field, which a Block can never represent (see ParamSpec.AsAttribute's
// doc comment in spec.go).
func TestRenderSchema_listOfObjectsAsAttribute(t *testing.T) {
	spec := TypeSpec{
		Kind:         "tag",
		TypeID:       "AsAttributeExample",
		Slug:         "asattributeexample",
		ResourceName: "matomo_tagmanager_tag_asattributeexample",
		Description:  "exercises the AsAttribute ListOfObjects override",
		Params: []ParamSpec{
			{
				MatomoName: "consentTypes", TFName: "consent_types", GoFieldName: "ConsentTypes",
				Description: "Consent types", GoType: "List", Required: false,
				IsListOfObjects: true,
				AsAttribute:     true,
				BlockName:       "consent_type",
				RowKeys: []RowKeySpec{
					{MatomoKey: "consent_type", TFName: "type", GoFieldName: "Type"},
					{MatomoKey: "consent_state", TFName: "state", GoFieldName: "State"},
				},
			},
		},
	}

	src, err := RenderSchema(spec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "tag_asattributeexample.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as valid Go: %v\n---\n%s", err, src)
	}

	got := string(src)
	for _, want := range []string{
		`"github.com/hashicorp/terraform-plugin-framework/attr"`,
		`"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"`,
		`"consent_type": schema.ListNestedAttribute{`,
		"PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}",
		"NestedObject: schema.NestedAttributeObject{",
		`p["consentTypes"] = matomo.ListOfObjectsParam(rows)`,
		`if !m.ConsentTypes.IsNull() && !m.ConsentTypes.IsUnknown() {`,
		"types.ObjectValueMust(attrTypes,",
		"types.ListValueMust(types.ObjectType{AttrTypes: attrTypes}, elements)",
		"types.ListNull(types.ObjectType{AttrTypes: attrTypes})",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q; full source:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Blocks: map[string]schema.Block{") {
		t.Errorf("AsAttribute param should never produce a Blocks map; full source:\n%s", got)
	}
	if strings.Contains(got, `"consent_type": schema.ListNestedBlock{`) {
		t.Errorf("consent_type should be an Attribute, not a Block; full source:\n%s", got)
	}
	// The whole reason AsAttribute exists is that a bare Go slice can't
	// represent an Unknown plan value (confirmed against a real
	// acceptance-test failure: "Received unknown value, however the
	// target type cannot handle unknown values"). The model field must
	// be types.List, not a generated row-struct slice, and no row struct
	// type should be emitted at all for an AsAttribute param.
	if !regexp.MustCompile(`ConsentTypes\s+types\.List`).MatchString(got) {
		t.Errorf("ConsentTypes model field should be types.List, not a row-struct slice; full source:\n%s", got)
	}
	if strings.Contains(got, "ConsentTypeModel struct {") {
		t.Errorf("AsAttribute param should not emit a row struct type; full source:\n%s", got)
	}
}
