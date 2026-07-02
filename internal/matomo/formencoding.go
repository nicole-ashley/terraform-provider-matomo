package matomo

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// stringMap decodes a field Matomo sometimes returns as an empty JSON array
// ([]) instead of an object ({}) - confirmed against a live instance for
// Trigger/Tag/Variable's "parameters" field. PHP's json_encode of an empty
// PHP array always produces [], since an empty array can't be distinguished
// from an empty list; a non-empty parameters map always serializes as a
// real object.
type stringMap map[string]string

func (m *stringMap) UnmarshalJSON(data []byte) error {
	var obj map[string]string
	if err := json.Unmarshal(data, &obj); err == nil {
		*m = obj
		return nil
	}
	var empty []any
	if err := json.Unmarshal(data, &empty); err != nil {
		return err
	}
	*m = map[string]string{}
	return nil
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
