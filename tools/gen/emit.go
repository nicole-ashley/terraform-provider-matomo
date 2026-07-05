// tools/gen/emit.go
package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"strings"
	"text/template"
)

// templateData adds the Go-identifier fields the template needs
// (GoModelName, GoSchemaFuncName, etc.) on top of a TypeSpec, without
// polluting TypeSpec itself with rendering-only concerns.
type templateData struct {
	TypeSpec
	GoModelName      string
	GoSchemaFuncName string
	GoTypeName       string
	GoModelReceiver  string
	// NeedsValidatorImports is true when the generated file actually
	// references stringvalidator/validator - tag types always do (the
	// common "status" attribute uses stringvalidator.OneOf), but
	// trigger/variable types only do when at least one of their own
	// parameters has AvailableValues. Importing these packages
	// unconditionally would produce an "imported and not used" compile
	// error on every trigger/variable type with no such parameter -
	// caught the hard way against real discovered types with no
	// AvailableValues params (e.g. PageView, DomReady).
	NeedsValidatorImports bool
	// CommonTypeName is the name of the hand-written typed{Tag,Trigger,
	// Variable}Common struct (internal/provider/typed_{tag,trigger,
	// variable}_resource.go) this kind's generated model anonymously
	// embeds, so a single req.Plan.Get/resp.State.Set call decodes or
	// encodes both the common fields and the type-specific ones together
	// (terraform-plugin-framework's reflection walks promoted fields from
	// embedded structs as if declared directly on the outer struct).
	CommonTypeName string
	// ModelInterfaceName is the per-kind interface (typedTagModel/
	// typedTriggerModel/typedVariableModel, internal/provider/
	// typed_model.go) the generated model's constructor returns, so the
	// shared runtime can call Common() without a type assertion.
	ModelInterfaceName string
	// NeedsTypesImport is true when the generated file actually
	// references the "types" package. Tag types always do (the common
	// fire_trigger_ids/block_trigger_ids attributes use
	// types.StringType), but trigger/variable types with zero
	// type-specific parameters (e.g. PageView, DomReady - the whole
	// model is just the embedded common struct) never reference it at
	// all, producing an "imported and not used" error - caught the hard
	// way against real discovered types with no parameters.
	NeedsTypesImport bool
	// NeedsBoolPlanModifierImport/NeedsInt64PlanModifierImport/
	// NeedsFloat64PlanModifierImport are true when at least one
	// non-Required parameter of that Go type exists - every non-Required
	// String/Bool/Int64/Float64 attribute is marked Optional+Computed
	// with a UseStateForUnknown plan modifier (see the Params loop in
	// schema.go.tmpl) so that a value Matomo defaults server-side (e.g.
	// Cookie's urlDecode coming back as false even though it was never
	// sent) doesn't leave Terraform reporting a perpetual "refresh plan
	// not empty" diff against the null a bare unset config produces -
	// confirmed as the single largest remaining acceptance-test failure
	// category in a live CI run, the same pattern the hand-written
	// "status" tag attribute already used for exactly this reason. List
	// is deliberately excluded (see block_trigger_ids' comment in
	// schema.go.tmpl): its generated Go field type, []types.String,
	// can't represent an unknown list at all, so Computed there produces
	// an outright decode failure rather than a diff - confirmed against
	// a real acceptance-test run.
	NeedsBoolPlanModifierImport    bool
	NeedsInt64PlanModifierImport   bool
	NeedsFloat64PlanModifierImport bool
	// GoModelBaseName is GoModelName without its "Model" suffix
	// (spec.Kind + ExportedName(spec.Slug), e.g.
	// "variableMatomoconfiguration") - the prefix a ListOfObjects
	// parameter's generated row struct type name is built from (e.g.
	// "variableMatomoconfigurationCustomDimensionModel"), matching this
	// file's existing unexported-struct naming convention.
	GoModelBaseName string
	// HasListOfObjectsBlocks is true when at least one parameter is
	// IsListOfObjects and NOT AsAttribute - the generated Schema() only
	// emits a Blocks: map when this is true, since a type with no such
	// parameter has nothing to put there. An AsAttribute ListOfObjects
	// parameter renders inside Attributes instead (see ParamSpec.AsAttribute).
	HasListOfObjectsBlocks bool
	// NeedsListPlanModifierImport is true when at least one parameter is
	// IsListOfObjects and AsAttribute - only a Computed ListNestedAttribute
	// needs listplanmodifier.UseStateForUnknown() (see ParamSpec.AsAttribute's
	// doc comment for why this shape exists at all).
	NeedsListPlanModifierImport bool
	// NeedsAttrImport is true when at least one parameter is
	// IsListOfObjects and AsAttribute - its model field is types.List
	// (not a bare Go slice, which can't represent an Unknown plan value -
	// confirmed against a real acceptance-test run: "Received unknown
	// value, however the target type cannot handle unknown values"), so
	// ToParams/FromParams build/read attr.Value objects by hand instead
	// of using terraform-plugin-framework's ctx-requiring ElementsAs/
	// ListValueFrom helpers (this codegen's ToParams/FromParams take no
	// context.Context, and adding one is a much larger ripple across
	// every generated file and the shared typed-resource runtime).
	NeedsAttrImport bool
}

func newTemplateData(spec TypeSpec) templateData {
	typeName := ExportedName(spec.Kind) + ExportedName(spec.Slug)
	needsValidatorImports := spec.Kind == "tag"
	if !needsValidatorImports {
		for _, p := range spec.Params {
			if len(p.AvailableValues) > 0 || p.ConditionallyRequired {
				needsValidatorImports = true
				break
			}
		}
	}
	needsInt64PM := spec.Kind == "tag" // priority is always an Int64 common attribute for tags
	var needsBoolPM, needsFloat64PM bool
	var hasListOfObjectsBlocks, needsListPM, needsAttrImport bool
	for _, p := range spec.Params {
		if !p.Required {
			switch p.GoType {
			case "Bool":
				needsBoolPM = true
			case "Int64":
				needsInt64PM = true
			case "Float64":
				needsFloat64PM = true
			}
		}
		if p.IsListOfObjects {
			if p.AsAttribute {
				needsListPM = true
				needsAttrImport = true
			} else {
				hasListOfObjectsBlocks = true
			}
		}
	}
	return templateData{
		TypeSpec:                       spec,
		GoModelName:                    spec.Kind + ExportedName(spec.Slug) + "Model",
		GoSchemaFuncName:               spec.Kind + ExportedName(spec.Slug) + "Schema",
		GoTypeName:                     typeName,
		GoModelReceiver:                "m",
		NeedsValidatorImports:          needsValidatorImports,
		CommonTypeName:                 "typed" + ExportedName(spec.Kind) + "Common",
		ModelInterfaceName:             "typed" + ExportedName(spec.Kind) + "Model",
		NeedsTypesImport:               spec.Kind == "tag" || len(spec.Params) > 0,
		NeedsBoolPlanModifierImport:    needsBoolPM,
		NeedsInt64PlanModifierImport:   needsInt64PM,
		NeedsFloat64PlanModifierImport: needsFloat64PM,
		GoModelBaseName:                spec.Kind + ExportedName(spec.Slug),
		HasListOfObjectsBlocks:         hasListOfObjectsBlocks,
		NeedsListPlanModifierImport:    needsListPM,
		NeedsAttrImport:                needsAttrImport,
	}
}

//go:embed templates/schema.go.tmpl
var schemaTemplateFS embed.FS

var schemaTemplate = template.Must(
	template.New("schema.go.tmpl").
		Funcs(template.FuncMap{"renderCondition": renderCondition, "lower": strings.ToLower, "pascal": SnakeToPascal}).
		ParseFS(schemaTemplateFS, "templates/schema.go.tmpl"),
)

// RenderSchema renders spec into a gofmt'd Go source file implementing
// the type's generated model + schema.Schema + typedModel methods, ready
// to write to internal/provider/generated/<kind>_<slug>.go.
func RenderSchema(spec TypeSpec) ([]byte, error) {
	var buf bytes.Buffer
	if err := schemaTemplate.Execute(&buf, newTemplateData(spec)); err != nil {
		return nil, fmt.Errorf("rendering template for %s %q: %w", spec.Kind, spec.TypeID, err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt-ing generated source for %s %q: %w\n--- unformatted source ---\n%s", spec.Kind, spec.TypeID, err, buf.String())
	}
	return formatted, nil
}
