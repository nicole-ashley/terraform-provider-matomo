package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// conditionEqualsValidator implements the Eq/Neq case of a generated
// type's Matomo `condition` expression: the attribute it's attached to is
// only meaningful when another attribute (Field) equals (or, if Negate,
// does not equal) Value. There's no built-in terraform-plugin-framework
// validator for "required/meaningful if sibling equals X," unlike the
// plain AlsoRequires/ConflictsWith cases a bare RefNode/NotNode maps to.
type conditionEqualsValidator struct {
	Field  string
	Value  string
	Negate bool
}

func (v conditionEqualsValidator) Description(_ context.Context) string {
	op := "=="
	if v.Negate {
		op = "!="
	}
	return fmt.Sprintf("only meaningful when %s %s %q", v.Field, op, v.Value)
}

func (v conditionEqualsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v conditionEqualsValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var fieldValue types.String
	diags := req.Config.GetAttribute(ctx, path.Root(v.Field), &fieldValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if fieldValue.IsNull() || fieldValue.IsUnknown() {
		return
	}

	matches := fieldValue.ValueString() == v.Value
	if v.Negate {
		matches = !matches
	}
	if !matches {
		op := "=="
		if v.Negate {
			op = "!="
		}
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid attribute combination",
			fmt.Sprintf("%s is only meaningful when %s %s %q", req.Path, v.Field, op, v.Value),
		)
	}
}

// conditionAnyOfValidator implements the Or case: the attribute it's
// attached to is valid if at least one wrapped validator reports no
// error (terraform-plugin-framework has no native "satisfy any of"
// combinator for validator.String).
type conditionAnyOfValidator struct {
	Validators []validator.String
}

func (v conditionAnyOfValidator) Description(ctx context.Context) string {
	return "must satisfy at least one condition"
}

func (v conditionAnyOfValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v conditionAnyOfValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	for _, inner := range v.Validators {
		var innerResp validator.StringResponse
		inner.ValidateString(ctx, req, &innerResp)
		if !innerResp.Diagnostics.HasError() {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid attribute combination",
		fmt.Sprintf("%s does not satisfy any of its required conditions", req.Path),
	)
}
