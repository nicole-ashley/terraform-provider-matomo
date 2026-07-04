package matomo

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"
)

func TestParamsMap_UnmarshalJSON(t *testing.T) {
	raw := `{
		"aString": "hello",
		"aBool": true,
		"aFalseBool": false,
		"aNumber": 42.5,
		"aCommaContainingString": "a,b,c",
		"aList": ["x", "y,z", "w"]
	}`

	var m ParamsMap
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := ParamsMap{
		"aString":                ScalarParam("hello"),
		"aBool":                  ScalarParam("1"),
		"aFalseBool":             ScalarParam("0"),
		"aNumber":                ScalarParam("42.5"),
		"aCommaContainingString": ScalarParam("a,b,c"),
		"aList":                  ListParam([]string{"x", "y,z", "w"}),
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("ParamsMap = %#v, want %#v", m, want)
	}

	// A comma inside a list element must survive intact - this is exactly
	// the case a comma-joined string encoding could never represent
	// correctly (no way to tell "y,z" as one element apart from "y" and
	// "z" as two).
	if got := m["aList"].List[1]; got != "y,z" {
		t.Errorf("aList[1] = %q, want \"y,z\" (comma preserved as a single element)", got)
	}
}

func TestParamsMap_UnmarshalJSON_emptyArray(t *testing.T) {
	// PHP's json_encode of an empty PHP array always produces [], not {} -
	// confirmed against a live instance for Trigger/Tag/Variable's
	// "parameters" field when it has no entries at all.
	var m ParamsMap
	if err := json.Unmarshal([]byte(`[]`), &m); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(m) != 0 {
		t.Errorf("ParamsMap = %#v, want empty", m)
	}
}

func TestAddParamsMap(t *testing.T) {
	m := ParamsMap{
		"scalar": ScalarParam("hello"),
		"list":   ListParam([]string{"a", "b,c"}),
	}
	v := url.Values{}
	addParamsMap(v, "parameters", m)

	if got := v.Get("parameters[scalar]"); got != "hello" {
		t.Errorf(`v.Get("parameters[scalar]") = %q, want "hello"`, got)
	}
	if _, ok := v["parameters[scalar][]"]; ok {
		t.Error(`v["parameters[scalar][]"] present, want a scalar entry to use the plain (non-array) key`)
	}

	// A list-typed entry must be sent as genuine repeated name[]=item
	// query parameters (Matomo's own PHP-array-from-query-string
	// convention), never as a single joined string - confirmed live:
	// a JSON-encoded array string was rejected outright with "... has
	// to be an array".
	gotList := v["parameters[list][]"]
	wantList := []string{"a", "b,c"}
	if !reflect.DeepEqual(gotList, wantList) {
		t.Errorf(`v["parameters[list][]"] = %#v, want %#v`, gotList, wantList)
	}
	if got := v.Get("parameters[list]"); got != "" {
		t.Errorf(`v.Get("parameters[list]") = %q, want "" (list values must not also be set under the bare key)`, got)
	}
}

func TestAddArrayParam(t *testing.T) {
	v := url.Values{}
	addArrayParam(v, "fireTriggerIds", []string{"1", "2", "3"})

	got := v["fireTriggerIds[]"]
	want := []string{"1", "2", "3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf(`v["fireTriggerIds[]"] = %#v, want %#v`, got, want)
	}
}

func TestParamValue_ListOfObjects(t *testing.T) {
	v := ListOfObjectsParam([]map[string]string{
		{"index": "1", "value": "foo"},
		{"index": "3", "value": "bar"},
	})
	if !v.IsListOfObjects() {
		t.Fatal("expected IsListOfObjects() to be true")
	}
	if v.IsList() {
		t.Fatal("expected IsList() to be false for a ListOfObjects value")
	}
}

func TestAddParamsMap_ListOfObjects(t *testing.T) {
	v := url.Values{}
	addParamsMap(v, "parameters", ParamsMap{
		"customDimensions": ListOfObjectsParam([]map[string]string{
			{"index": "1", "value": "foo"},
			{"index": "3", "value": "bar"},
		}),
	})
	want := url.Values{
		"parameters[customDimensions][0][index]": {"1"},
		"parameters[customDimensions][0][value]": {"foo"},
		"parameters[customDimensions][1][index]": {"3"},
		"parameters[customDimensions][1][value]": {"bar"},
	}
	if !reflect.DeepEqual(v, want) {
		t.Fatalf("got %#v, want %#v", v, want)
	}
}

func TestDecodeParamValue_ListOfObjects(t *testing.T) {
	var m ParamsMap
	raw := []byte(`{"customDimensions": [{"index": "1", "value": "foo"}, {"index": "3", "value": "bar"}]}`)
	if err := m.UnmarshalJSON(raw); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	v := m["customDimensions"]
	if !v.IsListOfObjects() {
		t.Fatal("expected IsListOfObjects() to be true")
	}
	want := []map[string]string{
		{"index": "1", "value": "foo"},
		{"index": "3", "value": "bar"},
	}
	if !reflect.DeepEqual(v.ListOfObjects, want) {
		t.Fatalf("got %#v, want %#v", v.ListOfObjects, want)
	}
}

func TestWrapSingleKeyParam(t *testing.T) {
	v := WrapSingleKeyParam("domain", []string{"*.example.com", "checkout.example.com"})
	if !v.IsListOfObjects() {
		t.Fatal("expected IsListOfObjects() to be true")
	}
	want := []map[string]string{
		{"domain": "*.example.com"},
		{"domain": "checkout.example.com"},
	}
	if !reflect.DeepEqual(v.ListOfObjects, want) {
		t.Fatalf("got %#v, want %#v", v.ListOfObjects, want)
	}
}
