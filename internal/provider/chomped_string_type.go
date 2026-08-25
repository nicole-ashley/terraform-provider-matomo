// internal/provider/chomped_string_type.go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// chompedStringType is the attribute type behind the free-form
// multi-line code fields users commonly configure via an HCL heredoc
// (variable_customjsfunction.js_function and tag_customhtml.custom_html -
// see trimTrailingNewlineOverrides in tools/gen/spec.go). It behaves
// exactly like types.StringType on the wire, and differs only in
// declaring that two values which agree after chompTrailingNewline are
// semantically equal.
//
// This has to be a custom type rather than a plan modifier because of
// where Terraform enforces consistency. A heredoc always folds its
// closing newline into the string value (only chomp() removes it), while
// Matomo doesn't retain that newline, so the value the provider reads
// back never matches the value the config produced. The obvious fix -
// having a plan modifier chomp the planned value - is rejected outright
// by Terraform: for an attribute that is not Computed, the planned value
// must equal either the config value or the prior state value, and a
// chomped value is neither on create. Terraform fails the plan with
// "Provider produced invalid plan ... planned value X does not match
// config value X\n" (confirmed against a real acceptance-test run).
//
// Semantic equality is the mechanism the framework provides for exactly
// this, and it acts at the three points that matter, in every case
// keeping the prior value when the two are semantically equal
// (see fwserver.SchemaSemanticEquality's callers):
//
//   - After Create and Update, comparing the planned value against the
//     value the provider put in state. Create reads the entity back from
//     Matomo before setting state (see typedTagResource.Create's
//     read-back comment), so without this the newline the plan carried is
//     lost and Terraform rejects the apply with "Provider produced
//     inconsistent result after apply" - which is what made a heredoc
//     value impossible to create at all.
//   - After Read, comparing prior state against the refreshed value, so
//     a heredoc-configured field doesn't report a perpetual diff on
//     every plan.
//
// Only a single trailing "\r\n" or "\n" is ignored, mirroring
// Terraform's own chomp() rather than trimming all trailing whitespace,
// so a value that legitimately ends with several blank lines still
// reports a real diff if the blank-line count changes.
type chompedStringType struct {
	basetypes.StringType
}

var _ basetypes.StringTypable = chompedStringType{}

func (t chompedStringType) String() string {
	return "provider.chompedStringType"
}

func (t chompedStringType) Equal(o attr.Type) bool {
	other, ok := o.(chompedStringType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t chompedStringType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return chompedString{StringValue: in}, nil
}

func (t chompedStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T, expected basetypes.StringValue", attrValue)
	}
	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}
	return stringValuable, nil
}

func (t chompedStringType) ValueType(_ context.Context) attr.Value {
	return chompedString{}
}

// chompedString is chompedStringType's value type.
type chompedString struct {
	basetypes.StringValue
}

var _ basetypes.StringValuableWithSemanticEquals = chompedString{}

func (v chompedString) Type(_ context.Context) attr.Type {
	return chompedStringType{}
}

// Equal stays strict (raw) equality - semantic equality is deliberately a
// separate question, asked only by the framework at the points listed in
// chompedStringType's doc comment.
func (v chompedString) Equal(o attr.Value) bool {
	other, ok := o.(chompedString)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

func (v chompedString) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(chompedString)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", v)+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", newValuable),
		)
		return false, diags
	}

	// The framework already skips the check when either side is null or
	// unknown (see fwschemadata.ValueSemanticEquality); guard anyway so a
	// null never compares equal to an empty string, which would silently
	// swallow a real change.
	if v.IsNull() != newValue.IsNull() || v.IsUnknown() != newValue.IsUnknown() {
		return false, diags
	}

	return chompTrailingNewline(v.ValueString()) == chompTrailingNewline(newValue.ValueString()), diags
}

func newChompedString(s string) chompedString {
	return chompedString{StringValue: basetypes.NewStringValue(s)}
}

func newChompedStringNull() chompedString {
	return chompedString{StringValue: basetypes.NewStringNull()}
}
