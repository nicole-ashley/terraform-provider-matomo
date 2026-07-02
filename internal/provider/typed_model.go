// internal/provider/typed_model.go
package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
