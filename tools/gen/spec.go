// tools/gen/spec.go
package main

import (
	"fmt"
	"sort"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// ParamSpec is one generated attribute, derived from a matomo.TemplateParam.
type ParamSpec struct {
	MatomoName      string
	TFName          string
	GoFieldName     string
	Description     string
	GoType          string // "String", "Bool", or "List"
	Required        bool
	AvailableValues []string
	Condition       matomo.ConditionNode
	// ConditionallyRequired is true when Matomo only requires this
	// parameter while Condition holds (see conditionallyRequiredParams
	// in tools/gen/required.go) - unlike Required, which is unconditional.
	// Always false when Condition is nil.
	ConditionallyRequired bool
}

// TypeSpec is one generated Tag Manager type (tag, trigger, or variable),
// ready to be rendered into Go source by Task 7's emitter.
type TypeSpec struct {
	Kind         string // "tag", "trigger", or "variable"
	TypeID       string // Matomo's type id, e.g. "CustomHtml"
	Slug         string // lowercased type id, e.g. "customhtml"
	ResourceName string // full Terraform type name, e.g. "matomo_tagmanager_tag_customhtml"
	Description  string
	Params       []ParamSpec
}

// matomoTypeToGoType maps Piwik\Settings\FieldConfig's TYPE_* constants
// (confirmed against core/Settings/FieldConfig.php: TYPE_INT='integer',
// TYPE_FLOAT='float', TYPE_STRING='string', TYPE_BOOL='boolean',
// TYPE_ARRAY='array' - not the shorthand names like "bool"/"int" the
// original design assumed, corrected against a live discovery response)
// to the Terraform attribute type used for the generated schema.
func matomoTypeToGoType(matomoType string) (string, error) {
	switch matomoType {
	case "string":
		return "String", nil
	case "boolean":
		return "Bool", nil
	case "integer":
		return "Int64", nil
	case "float":
		return "Float64", nil
	case "array":
		return "List", nil
	default:
		return "", fmt.Errorf("unrecognized Matomo parameter type %q - add a case to matomoTypeToGoType in tools/gen/spec.go", matomoType)
	}
}

// BuildTypeSpec converts one discovered Matomo template into a TypeSpec,
// consulting RequiredParams (Task 4) for required-ness and ParseCondition
// (Task 3) for each parameter's condition. kind must be "tag", "trigger",
// or "variable".
func BuildTypeSpec(kind string, tmpl matomo.Template) (TypeSpec, error) {
	required, err := RequiredParams(kind, tmpl.ID)
	if err != nil {
		return TypeSpec{}, err
	}
	requiredSet := make(map[string]bool, len(required))
	for _, name := range required {
		requiredSet[name] = true
	}

	condRequiredSet := make(map[string]bool, len(ConditionallyRequiredParams(kind, tmpl.ID)))
	for _, name := range ConditionallyRequiredParams(kind, tmpl.ID) {
		condRequiredSet[name] = true
	}

	spec := TypeSpec{
		Kind:         kind,
		TypeID:       tmpl.ID,
		Slug:         Slug(tmpl.ID),
		ResourceName: fmt.Sprintf("matomo_tagmanager_%s_%s", kind, Slug(tmpl.ID)),
		Description:  tmpl.Description,
	}

	for _, p := range tmpl.Parameters {
		goType, err := matomoTypeToGoType(p.Type)
		if err != nil {
			return TypeSpec{}, fmt.Errorf("type %q, parameter %q: %w", tmpl.ID, p.Name, err)
		}
		cond, err := matomo.ParseCondition(p.Condition)
		if err != nil {
			return TypeSpec{}, fmt.Errorf("type %q, parameter %q: %w", tmpl.ID, p.Name, err)
		}
		if cond != nil {
			cond = rewriteConditionFieldsToTFNames(cond)
		}

		conditionallyRequired := condRequiredSet[p.Name]
		if conditionallyRequired && cond == nil {
			return TypeSpec{}, fmt.Errorf("type %q, parameter %q: listed in conditionallyRequiredParams but has no condition to require it under - check tools/gen/required.go", tmpl.ID, p.Name)
		}
		if conditionallyRequired && goType != "String" {
			return TypeSpec{}, fmt.Errorf("type %q, parameter %q: conditionallyRequiredParams only supports String parameters today, got GoType %q - extend conditionRequiredValidator in internal/provider/condition_validators.go first", tmpl.ID, p.Name, goType)
		}

		var availableValues []string
		for value := range p.AvailableValues {
			availableValues = append(availableValues, value)
		}
		sort.Strings(availableValues)

		spec.Params = append(spec.Params, ParamSpec{
			MatomoName:            p.Name,
			TFName:                CamelToSnake(p.Name),
			GoFieldName:           ExportedName(p.Name),
			Description:           p.Description,
			GoType:                goType,
			Required:              requiredSet[p.Name],
			AvailableValues:       availableValues,
			Condition:             cond,
			ConditionallyRequired: conditionallyRequired,
		})
	}

	return spec, nil
}

// rewriteConditionFieldsToTFNames translates a parsed condition's field
// references from Matomo's camelCase parameter names ("triggerType") into
// the snake_case Terraform attribute names ("trigger_type") the generated
// schema actually uses. Doing this once at codegen time means the runtime
// evaluator (internal/matomo.Evaluate, called from
// internal/provider/condition_validators.go) can look sibling values up by
// Terraform attribute path directly, with no naming logic of its own.
func rewriteConditionFieldsToTFNames(node matomo.ConditionNode) matomo.ConditionNode {
	switch n := node.(type) {
	case matomo.RefNode:
		return matomo.RefNode{Field: CamelToSnake(n.Field)}
	case matomo.EqNode:
		return matomo.EqNode{Field: CamelToSnake(n.Field), Value: n.Value, Negate: n.Negate}
	case matomo.NotNode:
		return matomo.NotNode{Inner: rewriteConditionFieldsToTFNames(n.Inner)}
	case matomo.AndNode:
		return matomo.AndNode{Left: rewriteConditionFieldsToTFNames(n.Left), Right: rewriteConditionFieldsToTFNames(n.Right)}
	case matomo.OrNode:
		return matomo.OrNode{Left: rewriteConditionFieldsToTFNames(n.Left), Right: rewriteConditionFieldsToTFNames(n.Right)}
	default:
		return node
	}
}
