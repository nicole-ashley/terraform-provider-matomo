package matomo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// ParamValue is one entry in a Tag/Trigger/Variable's flat "parameters"
// map: a plain scalar, a list, or a list of objects. Matomo's own
// dispatcher builds PHP arrays for array-typed parameters from the raw
// query string's native name[]=x / name[i][key]=x convention, not from a
// JSON-encoded string value - confirmed against a live instance (a
// JSON-encoded array string was rejected outright with "<Field>: ... has to
// be an array"). List is non-nil (even if zero-length) exactly when a value
// represents a flat list of scalars; ListOfObjects is non-nil exactly when
// it represents a list of rows, each with its own named string fields
// (Matomo's UI_CONTROL_MULTI_TUPLE parameters - e.g. MatomoConfiguration's
// customDimensions is a list of {index, value} rows, confirmed against
// Matomo's own MatomoConfigurationVariable.php source, not merely a flat
// list of strings the way ListParam alone could represent).
type ParamValue struct {
	Scalar        string
	List          []string
	ListOfObjects []map[string]string
}

// ScalarParam builds a plain scalar ParamValue.
func ScalarParam(s string) ParamValue { return ParamValue{Scalar: s} }

// ListParam builds a list-typed ParamValue.
func ListParam(items []string) ParamValue { return ParamValue{List: items} }

// IsList reports whether v represents a list-typed value.
func (v ParamValue) IsList() bool { return v.List != nil }

// ListOfObjectsParam builds a list-of-objects-typed ParamValue.
func ListOfObjectsParam(rows []map[string]string) ParamValue { return ParamValue{ListOfObjects: rows} }

// IsListOfObjects reports whether v represents a list-of-objects-typed value.
func (v ParamValue) IsListOfObjects() bool { return v.ListOfObjects != nil }

// WrapSingleKeyParam builds a ListOfObjects-typed ParamValue from a flat
// list of strings, wrapping each one as a single-key row - for parameters
// like MatomoConfiguration's domains, which Matomo's PHP source declares as
// UI_CONTROL_MULTI_TUPLE (so it needs the same [i][key]=value wire shape as
// any other list-of-objects parameter) but whose rows only ever have one
// key, so there's no pairing information a nested Terraform block would add
// over a flat list(string) - this keeps the Terraform-facing schema flat
// while still sending Matomo the shape it expects.
func WrapSingleKeyParam(key string, items []string) ParamValue {
	rows := make([]map[string]string, len(items))
	for i, item := range items {
		rows[i] = map[string]string{key: item}
	}
	return ListOfObjectsParam(rows)
}

// ParamsMap is the "parameters" field shared by Tag/Trigger/Variable,
// keyed by Matomo's own parameter name.
type ParamsMap map[string]ParamValue

// UnmarshalJSON decodes a field Matomo sometimes returns as an empty
// JSON array ([]) instead of an object ({}) - confirmed against a live
// instance for Trigger/Tag/Variable's "parameters" field. PHP's
// json_encode of an empty PHP array always produces [], since an empty
// array can't be distinguished from an empty list; a non-empty
// parameters map always serializes as a real object.
//
// Also confirmed against live typed-resource acceptance runs: not every
// value in that object is a JSON string. A Bool- or Int64/Float64-typed
// Tag Manager parameter (e.g. a checkbox or number field) round-trips as
// a real JSON boolean/number, not the string PHP's own admin-UI form
// submission would send; a TYPE_ARRAY parameter round-trips as a real
// JSON array. Each value is decoded to the matching ParamValue shape.
func (m *ParamsMap) UnmarshalJSON(data []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		out := make(ParamsMap, len(obj))
		for k, raw := range obj {
			out[k] = decodeParamValue(raw)
		}
		*m = out
		return nil
	}
	var empty []any
	if err := json.Unmarshal(data, &empty); err != nil {
		return err
	}
	*m = ParamsMap{}
	return nil
}

// decodeParamValue decodes one raw JSON value from a "parameters"
// object into the flat ParamValue representation the rest of this
// provider works with. Matches the encodings paramBoolString/
// paramInt64String/paramFloat64String (internal/provider/typed_model.go)
// produce for scalars, so a value read back from Matomo round-trips
// through FromParams/ToParams unchanged.
func decodeParamValue(raw json.RawMessage) ParamValue {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return ScalarParam(s)
	}
	var objArr []map[string]string
	if err := json.Unmarshal(raw, &objArr); err == nil && len(objArr) > 0 {
		return ListOfObjectsParam(objArr)
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return ListParam(arr)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		if b {
			return ScalarParam("1")
		}
		return ScalarParam("0")
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return ScalarParam(strconv.FormatFloat(f, 'f', -1, 64))
	}
	// null, or some other shape no discovered type has hit yet - treat
	// as an unset scalar rather than losing the value silently.
	return ScalarParam("")
}

// Matomo's API dispatcher builds PHP arrays for array-typed parameters from
// the raw query string itself (PHP's native name[]=x&name[]=y / nested
// name[key]=value convention), not from a JSON-encoded string value -
// confirmed against a live instance (JSON-encoded fireTriggerIds/conditions/
// parameters values were rejected with "<Field>: ... has to be an array" /
// "Value: A value needs to be provided."). These helpers build query
// parameters in that native form.

// addArrayParam sets name[] for each item in a list-typed parameter.
func addArrayParam(v url.Values, name string, items []string) {
	for _, item := range items {
		v.Add(name+"[]", item)
	}
}

// addParamsMap sets name[key] for each scalar entry, and name[key][] for
// each item of a list entry, in a Tag/Trigger/Variable's "parameters" map -
// see ParamValue's doc comment for why a list-typed entry can't just be a
// joined string.
func addParamsMap(v url.Values, name string, m ParamsMap) {
	for key, val := range m {
		if val.IsListOfObjects() {
			for i, row := range val.ListOfObjects {
				for rowKey, rowValue := range row {
					v.Set(fmt.Sprintf("%s[%s][%d][%s]", name, key, i, rowKey), rowValue)
				}
			}
			continue
		}
		if val.IsList() {
			for _, item := range val.List {
				v.Add(fmt.Sprintf("%s[%s][]", name, key), item)
			}
			continue
		}
		v.Set(fmt.Sprintf("%s[%s]", name, key), val.Scalar)
	}
}

// addConditionsParam sets name[i][field] for each condition in a list of
// trigger conditions.
func addConditionsParam(v url.Values, name string, conditions []Condition) {
	for i, cond := range conditions {
		prefix := fmt.Sprintf("%s[%d]", name, i)
		v.Set(prefix+"[comparison]", cond.Comparison)
		v.Set(prefix+"[actual]", cond.ActualValueVariableID)
		v.Set(prefix+"[expected]", cond.ExpectedValue)
	}
}
