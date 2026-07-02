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
// Matomo-specific parameters - the common fields shared by every tag (or
// trigger, or variable) are declared identically by every generated
// model (same tfsdk tags, same order) and are read directly by the
// shared runtime via req.Plan.Get/resp.State.Set against that common
// schema shape, not through this interface.
type typedModel interface {
	Meta() typedMeta
	ToParams() map[string]string
	FromParams(params map[string]string)
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
