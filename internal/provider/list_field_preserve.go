// internal/provider/list_field_preserve.go
package provider

import (
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringListFieldType is the Go type every generated List-typed model
// field uses (see tools/gen/templates/schema.go.tmpl).
var stringListFieldType = reflect.TypeOf([]types.String{})

// snapshotListFields walks model (a pointer to a generated typedModel,
// which anonymously embeds a typed{Tag,Trigger,Variable}Common struct)
// and captures every []types.String field, keyed by its tfsdk tag.
//
// List-typed attributes are deliberately NOT marked Computed (see
// block_trigger_ids' comment in schema.go.tmpl: the generated
// []types.String field type has no way to represent "the whole list is
// unknown," so Computed there fails outright rather than just diffing).
// That means terraform-plugin-framework requires a List attribute's
// value after Create/Update to exactly match what the plan already
// computed - overwriting it with FromParams(realData)'s freshly-read
// value (done to resolve the *other*, now-Computed fields out of
// Unknown) can silently diverge from the plan whenever Matomo defaults
// an unset field to something non-empty, which Terraform then rejects as
// "provider produced inconsistent result after apply" - a hard failure,
// confirmed against a real acceptance-test run for
// EtrackerConfiguration's custom_dimensions. snapshotListFields/
// restoreListFields bracket that FromParams call so List fields end up
// back at their plan-consistent value regardless of what FromParams
// just set them to.
func snapshotListFields(model typedModel) map[string][]types.String {
	saved := map[string][]types.String{}
	walkListFields(reflect.ValueOf(model).Elem(), func(tag string, v reflect.Value) {
		cur, _ := v.Interface().([]types.String)
		saved[tag] = append([]types.String(nil), cur...)
	})
	return saved
}

func restoreListFields(model typedModel, saved map[string][]types.String) {
	walkListFields(reflect.ValueOf(model).Elem(), func(tag string, v reflect.Value) {
		if val, ok := saved[tag]; ok {
			v.Set(reflect.ValueOf(val))
		}
	})
}

func walkListFields(v reflect.Value, fn func(tag string, v reflect.Value)) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)
		if field.Anonymous && fv.Kind() == reflect.Struct {
			walkListFields(fv, fn)
			continue
		}
		if fv.Type() == stringListFieldType {
			fn(field.Tag.Get("tfsdk"), fv)
		}
	}
}
