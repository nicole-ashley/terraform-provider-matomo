// internal/provider/typed_model.go
package provider

import (
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// typedMeta is the per-type metadata a generated model supplies to the
// shared typed{Tag,Trigger,Variable}Resource runtime.
type typedMeta struct {
	TypeID       string // Matomo's type id, e.g. "CustomHtml"
	ResourceName string // full Terraform type name, e.g. "matomo_tagmanager_tag_customhtml"
	Schema       schema.Schema
}

// typedModel is satisfied by every generated model type (Task 7's
// emitter output). ToParams/FromParams only handle a type's
// Matomo-specific parameters.
type typedModel interface {
	Meta() typedMeta
	ToParams() map[string]string
	FromParams(params map[string]string)
}

// typedTagModel/typedTriggerModel/typedVariableModel are satisfied by
// every generated model of that kind. Each generated model struct
// anonymously embeds the matching typed{Tag,Trigger,Variable}Common
// struct (see tools/gen/templates/schema.go.tmpl), so a single
// req.Plan.Get(ctx, model)/resp.State.Set(ctx, model) call decodes or
// encodes both the common fields and the type-specific ones together in
// one pass - terraform-plugin-framework's reflection walks promoted
// fields from embedded structs as if they were declared directly on the
// outer struct. Common() returns a pointer into that same embedded
// struct, so mutations the shared CRUD runtime makes (setting a new id
// after create, a value read back from Matomo, etc.) are mutations of
// the model itself, not a separate copy.
//
// This replaced an earlier design (a second, disjoint Get/Set call
// against a bare typed*Common struct) that failed at the first real live
// Matomo run: terraform-plugin-framework's Get() requires the
// destination struct to have a field for every attribute in the
// schema it's being decoded against, so decoding a full tag/trigger/
// variable object into just its common subset always errored with a
// "mismatch between struct and object" diagnostic.
type typedTagModel interface {
	typedModel
	Common() *typedTagCommon
}

type typedTriggerModel interface {
	typedModel
	Common() *typedTriggerCommon
}

type typedVariableModel interface {
	typedModel
	Common() *typedVariableCommon
}

// Matomo's Tag Manager "parameters" wire field is always a flat
// map[string]string (confirmed against real create/read responses earlier
// in this project), regardless of a parameter's declared Terraform type -
// these helpers convert non-string typed-model fields to/from that string
// representation. Generated ToParams()/FromParams() methods (Task 7's
// emitter output) call these for Bool/Int64/Float64/List parameters.
//
// Bool encoding ("1"/"0") is a reasonable first guess based on common PHP
// form-checkbox convention, not yet confirmed against a live Matomo
// boolean parameter round-trip - if a live acceptance test for a
// boolean-parameter type fails with a value mismatch, this is the first
// place to check.
func paramBoolString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func paramBoolValue(s string) bool {
	return s == "1" || s == "true"
}

func paramInt64String(n int64) string {
	return strconv.FormatInt(n, 10)
}

func paramInt64Value(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func paramFloat64String(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func paramFloat64Value(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// paramListString/paramListValue's comma-joined encoding is likewise an
// unconfirmed first guess for how Matomo represents an array-typed Tag
// Manager parameter within the flat parameters map - see the Bool note
// above for the same caveat.
func paramListString(list []types.String) string {
	return strings.Join(stringSliceFromModel(list), ",")
}

func paramListValue(s string) []types.String {
	if s == "" {
		return nil
	}
	return stringModelFromSlice(strings.Split(s, ","))
}
