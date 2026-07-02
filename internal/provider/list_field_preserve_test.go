// internal/provider/list_field_preserve_test.go
package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fakeListModel struct {
	typedTagCommon
	Items []types.String `tfsdk:"items"`
}

func (m *fakeListModel) Meta() typedMeta { return typedMeta{} }
func (m *fakeListModel) ToParams() map[string]string {
	return nil
}
func (m *fakeListModel) FromParams(map[string]string) {}
func (m *fakeListModel) Common() *typedTagCommon      { return &m.typedTagCommon }

var _ typedTagModel = &fakeListModel{}

// TestSnapshotAndRestoreListFields proves the reflection walk finds both
// the type-specific List field ("items") and the embedded common struct's
// List fields (fire_trigger_ids/block_trigger_ids), and that restoring a
// snapshot undoes any mutation made in between - the exact bracket
// pattern typed_tag_resource.go's Create uses around model.FromParams to
// keep List attributes (deliberately not Computed) consistent with the
// plan after a live read-back.
func TestSnapshotAndRestoreListFields(t *testing.T) {
	m := &fakeListModel{}
	m.FireTriggerIDs = []types.String{types.StringValue("1"), types.StringValue("2")}
	m.BlockTriggerIDs = nil
	m.Items = []types.String{types.StringValue("a")}

	saved := snapshotListFields(m)

	if len(saved) != 3 {
		t.Fatalf("snapshotListFields() captured %d fields, want 3 (fire_trigger_ids, block_trigger_ids, items): %v", len(saved), saved)
	}

	// Mutate every List field, simulating FromParams overwriting them
	// with freshly-read Matomo data.
	m.FireTriggerIDs = []types.String{types.StringValue("mutated")}
	m.BlockTriggerIDs = []types.String{types.StringValue("mutated")}
	m.Items = []types.String{types.StringValue("mutated")}

	restoreListFields(m, saved)

	if !reflect.DeepEqual(m.FireTriggerIDs, []types.String{types.StringValue("1"), types.StringValue("2")}) {
		t.Errorf("FireTriggerIDs = %v after restore, want original [1 2]", m.FireTriggerIDs)
	}
	if m.BlockTriggerIDs != nil {
		t.Errorf("BlockTriggerIDs = %v after restore, want nil (the original value)", m.BlockTriggerIDs)
	}
	if !reflect.DeepEqual(m.Items, []types.String{types.StringValue("a")}) {
		t.Errorf("Items = %v after restore, want original [a]", m.Items)
	}
}
