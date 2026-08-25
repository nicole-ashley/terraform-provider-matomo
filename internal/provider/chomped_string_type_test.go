package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The semantic equality that lets a heredoc value survive a create: the
// framework compares the planned value (prior) against the value the
// provider read back from Matomo (new) and keeps the planned one when
// these agree, so state matches the plan and Terraform doesn't reject the
// apply. The same check keeps a refresh from reporting a perpetual diff.
func TestChompedString_semanticEqualsIgnoresOneTrailingNewline(t *testing.T) {
	tests := []struct {
		name  string
		prior string
		new   string
		want  bool
	}{
		{"heredoc value vs Matomo's chomped read-back", "function() { return 1; }\n", "function() { return 1; }", true},
		{"chomped prior vs heredoc new", "function() { return 1; }", "function() { return 1; }\n", true},
		{"carriage return newline", "function() { return 1; }\r\n", "function() { return 1; }", true},
		{"identical values", "function() { return 1; }", "function() { return 1; }", true},
		{"a real change is not equal", "function() { return 1; }\n", "function() { return 2; }", false},
		{"only one trailing newline is ignored", "function() { return 1; }\n\n", "function() { return 1; }", false},
		{"trailing whitespace is not ignored", "function() { return 1; } \n", "function() { return 1; }", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prior := newChompedString(tt.prior)
			got, diags := newChompedString(tt.new).StringSemanticEquals(context.Background(), prior)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tt.want {
				t.Errorf("StringSemanticEquals(%q, %q) = %v, want %v", tt.new, tt.prior, got, tt.want)
			}
		})
	}
}

// A null must never compare equal to an empty string - both chomp to "",
// so without an explicit guard a real change would be silently swallowed.
func TestChompedString_semanticEqualsNullIsNotEmptyString(t *testing.T) {
	got, diags := newChompedString("").StringSemanticEquals(context.Background(), newChompedStringNull())
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got {
		t.Error("StringSemanticEquals(null, \"\") = true, want false")
	}
}

func TestChompedString_semanticEqualsWrongTypeErrors(t *testing.T) {
	_, diags := newChompedString("x").StringSemanticEquals(context.Background(), basetypes.NewStringValue("x"))
	if !diags.HasError() {
		t.Error("StringSemanticEquals with a non-chompedString value returned no error diagnostic")
	}
}

// Equal stays strict: semantic equality is a separate question, asked only
// by the framework at the points chompedStringType documents. If Equal were
// semantic too, Terraform would never see a trailing-newline change at all.
func TestChompedString_equalIsStrict(t *testing.T) {
	if newChompedString("a\n").Equal(newChompedString("a")) {
		t.Error("Equal(\"a\\n\", \"a\") = true, want false (Equal must stay raw equality)")
	}
	if !newChompedString("a").Equal(newChompedString("a")) {
		t.Error("Equal(\"a\", \"a\") = false, want true")
	}
	if newChompedString("a").Equal(basetypes.NewStringValue("a")) {
		t.Error("Equal(chompedString, basetypes.StringValue) = true, want false (different value types)")
	}
}

// The type has to round-trip through tftypes as a plain string, so it is
// wire-compatible with the types.String it replaces - a stored state
// written before this type existed still decodes.
func TestChompedStringType_valueFromTerraform(t *testing.T) {
	ctx := context.Background()

	typ := chompedStringType{}

	got, err := typ.ValueFromTerraform(ctx, tftypes.NewValue(tftypes.String, "function() { return 1; }\n"))
	if err != nil {
		t.Fatalf("ValueFromTerraform: %v", err)
	}
	want := newChompedString("function() { return 1; }\n")
	if !got.Equal(want) {
		t.Errorf("ValueFromTerraform = %#v, want %#v", got, want)
	}

	gotNull, err := typ.ValueFromTerraform(ctx, tftypes.NewValue(tftypes.String, nil))
	if err != nil {
		t.Fatalf("ValueFromTerraform(null): %v", err)
	}
	if !gotNull.IsNull() {
		t.Errorf("ValueFromTerraform(null) = %#v, want a null value", gotNull)
	}

	if tfType := typ.TerraformType(ctx); !tfType.Equal(tftypes.String) {
		t.Errorf("TerraformType() = %v, want tftypes.String", tfType)
	}
}

func TestChompedStringType_equalAndValueType(t *testing.T) {
	typ := chompedStringType{}

	if !typ.Equal(chompedStringType{}) {
		t.Error("chompedStringType is not Equal to itself")
	}
	if typ.Equal(basetypes.StringType{}) {
		t.Error("chompedStringType.Equal(basetypes.StringType) = true, want false")
	}
	// The framework builds a zero value from ValueType when decoding; it
	// must be a null chompedString, not an empty-string one.
	zero := typ.ValueType(context.Background())
	if _, ok := zero.(chompedString); !ok {
		t.Fatalf("ValueType() = %T, want chompedString", zero)
	}
	if !zero.IsNull() {
		t.Errorf("ValueType() = %#v, want a null value", zero)
	}
}
