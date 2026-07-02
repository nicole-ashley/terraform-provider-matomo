package matomo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// stringMap decodes a field Matomo sometimes returns as an empty JSON array
// ([]) instead of an object ({}) - confirmed against a live instance for
// Trigger/Tag/Variable's "parameters" field. PHP's json_encode of an empty
// PHP array always produces [], since an empty array can't be distinguished
// from an empty list; a non-empty parameters map always serializes as a
// real object.
//
// Also confirmed against live typed-resource acceptance runs: not every
// value in that object is a JSON string. A Bool- or Int64/Float64-typed
// Tag Manager parameter (e.g. a checkbox or number field) round-trips as
// a real JSON boolean/number, not the string PHP's own admin-UI form
// submission would send - so a strict map[string]string decode fails for
// any type with such a parameter. Falls back to a lenient
// map[string]any decode, stringifying each value, before finally falling
// back to the empty-array case above.
type stringMap map[string]string

func (m *stringMap) UnmarshalJSON(data []byte) error {
	var obj map[string]string
	if err := json.Unmarshal(data, &obj); err == nil {
		*m = obj
		return nil
	}
	var objAny map[string]any
	if err := json.Unmarshal(data, &objAny); err == nil {
		out := make(map[string]string, len(objAny))
		for k, v := range objAny {
			out[k] = stringifyParamValue(v)
		}
		*m = out
		return nil
	}
	var empty []any
	if err := json.Unmarshal(data, &empty); err != nil {
		return err
	}
	*m = map[string]string{}
	return nil
}

// stringifyParamValue converts one decoded JSON parameter value (from the
// map[string]any fallback above) into the flat string representation the
// rest of this provider works with. Matches the encodings
// paramBoolString/paramInt64String/paramFloat64String
// (internal/provider/typed_model.go) produce, so a value read back from
// Matomo round-trips through FromParams/ToParams unchanged.
func stringifyParamValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "1"
		}
		return "0"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return ""
	default:
		// An array/object value (e.g. a genuinely array-typed parameter,
		// distinct from the whole-field-is-an-empty-array case handled
		// above) - fall back to its JSON form rather than losing the
		// value entirely; this is not yet confirmed against a live
		// example, since no discovered type has hit this path.
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	}
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

// addMapParam sets name[key] for each entry in a map-typed parameter.
func addMapParam(v url.Values, name string, m map[string]string) {
	for key, val := range m {
		v.Set(fmt.Sprintf("%s[%s]", name, key), val)
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
