package matomo

import (
	"fmt"
	"net/url"
)

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
