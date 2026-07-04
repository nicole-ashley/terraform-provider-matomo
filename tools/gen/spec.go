// tools/gen/spec.go
package main

import (
	"encoding/json"
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
	// IsListOfObjects is true when this GoType=="List" parameter is
	// really a multi-key UI_CONTROL_MULTI_TUPLE parameter (2+ row keys,
	// e.g. customDimensions' {index, value}) - it gets a real nested
	// block instead of a flat schema.ListAttribute. BlockName and
	// RowKeys are only populated when this is true.
	IsListOfObjects bool
	// BlockName is the Terraform block name for a ListOfObjects param's
	// rows - the singular form of TFName (e.g. "custom_dimension" for
	// "custom_dimensions"), per this project's naming convention.
	BlockName string
	// RowKeys is the ordered list of this ListOfObjects param's row
	// keys, as confirmed by Matomo's own uiControlAttributes.
	RowKeys []RowKeySpec
	// SingleKeyName is the one row key of a single-key
	// UI_CONTROL_MULTI_TUPLE parameter (e.g. "domain" for domains) - the
	// Go field/schema stay a flat List (matomoTypeToGoType still maps
	// this to "List"), only the wire encoding differs
	// (matomo.WrapSingleKeyParam instead of matomo.ListParam). Empty for
	// every other List parameter, including IsListOfObjects ones.
	SingleKeyName string
	// AsAttribute is true when an IsListOfObjects parameter must be
	// emitted as a Computed schema.ListNestedAttribute instead of a
	// schema.ListNestedBlock (see listOfObjectsAsAttributeOverrides for
	// why - only set via that override table, never auto-detected).
	// Meaningless when IsListOfObjects is false.
	AsAttribute bool
}

// listOfObjectsAsAttributeOverrides marks parameters whose ListOfObjects
// shape must be emitted as a Computed schema.ListNestedAttribute instead
// of a schema.ListNestedBlock, keyed by "kind/typeID/matomoParamName".
// This is required whenever Matomo defines a non-empty server-side
// default for the field - a Terraform Block's cardinality is dictated
// entirely by what's in the user's config (there is no Computed concept
// for blocks), so a provider can never legally return more block
// instances than were configured; this hard-fails
// "Provider produced inconsistent result after apply" the moment Matomo
// fills in a default the user didn't ask for. Confirmed via a live
// acceptance-test failure + Matomo's own PHP source
// (GoogleConsentModeV2Tag.php's consentTypes defaults to all 7 real
// consent keys, state "granted", whenever unset) - every other
// ListOfObjects field defaults to empty/absent and stays a Block.
var listOfObjectsAsAttributeOverrides = map[string]bool{
	"tag/GoogleConsentModeV2/consentTypes": true,
}

// RowKeySpec is one named sub-field of a ListOfObjects parameter's rows,
// e.g. customDimensions' "index" or "value".
type RowKeySpec struct {
	MatomoKey   string // raw Matomo row key, e.g. "index"
	TFName      string // Terraform-facing attribute name (rowKeyTFName applies any override)
	GoFieldName string // Go field name for the generated row struct, e.g. "Index"
}

// multiTupleRowKeys returns a UI_CONTROL_MULTI_TUPLE parameter's row keys
// in their real, defined order (Matomo's uiControlAttributes is a map
// keyed "field1", "field2", ... - map iteration order is not the row's
// real field order, so this scans sequentially instead and stops at the
// first missing fieldN, which also naturally excludes any non-field
// keys a future Matomo version might add alongside field1/field2).
func multiTupleRowKeys(attrs matomo.UIControlAttributeValues) []string {
	var keys []string
	for i := 1; ; i++ {
		raw, ok := attrs[fmt.Sprintf("field%d", i)]
		if !ok {
			break
		}
		var field matomo.UIControlField
		if err := json.Unmarshal(raw, &field); err != nil || field.Key == "" {
			break
		}
		keys = append(keys, field.Key)
	}
	return keys
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

		var isListOfObjects bool
		var blockName string
		var rowKeys []RowKeySpec
		var singleKeyName string
		var asAttribute bool
		if goType == "List" && p.UIControl == "multituple" {
			rawKeys := multiTupleRowKeys(p.UIControlAttributes)
			switch {
			case len(rawKeys) > 1:
				isListOfObjects = true
				blockName = Singularize(CamelToSnake(p.Name))
				asAttribute = listOfObjectsAsAttributeOverrides[kind+"/"+tmpl.ID+"/"+p.Name]
				for _, k := range rawKeys {
					tfKey := rowKeyTFName(kind, tmpl.ID, p.Name, k)
					rowKeys = append(rowKeys, RowKeySpec{
						MatomoKey:   k,
						TFName:      tfKey,
						GoFieldName: SnakeToPascal(tfKey),
					})
				}
			case len(rawKeys) == 1:
				singleKeyName = rawKeys[0]
			}
		}

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
			IsListOfObjects:       isListOfObjects,
			BlockName:             blockName,
			RowKeys:               rowKeys,
			SingleKeyName:         singleKeyName,
			AsAttribute:           asAttribute,
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
