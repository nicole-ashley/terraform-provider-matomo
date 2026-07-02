package matomo

import (
	"reflect"
	"testing"
)

func TestParseCondition(t *testing.T) {
	cases := []struct {
		expr string
		want ConditionNode
	}{
		{"sitesearch", RefNode{Field: "sitesearch"}},
		{"!sitesearch", NotNode{Inner: RefNode{Field: "sitesearch"}}},
		{
			"sitesearch && !use_sitesearch_default",
			AndNode{
				Left:  RefNode{Field: "sitesearch"},
				Right: NotNode{Inner: RefNode{Field: "use_sitesearch_default"}},
			},
		},
		{
			"a || b",
			OrNode{Left: RefNode{Field: "a"}, Right: RefNode{Field: "b"}},
		},
		{
			`triggerType == "pageview"`,
			EqNode{Field: "triggerType", Value: "pageview", Negate: false},
		},
		{
			`triggerType != 'pageview'`,
			EqNode{Field: "triggerType", Value: "pageview", Negate: true},
		},
	}

	for _, c := range cases {
		got, err := ParseCondition(c.expr)
		if err != nil {
			t.Fatalf("ParseCondition(%q) error = %v", c.expr, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseCondition(%q) = %#v, want %#v", c.expr, got, c.want)
		}
	}
}

func TestParseCondition_unparseable(t *testing.T) {
	_, err := ParseCondition("a &&")
	if err == nil {
		t.Fatal("ParseCondition(\"a &&\") error = nil, want a parse error")
	}
}

func TestParseCondition_empty(t *testing.T) {
	got, err := ParseCondition("")
	if err != nil {
		t.Fatalf("ParseCondition(\"\") error = %v", err)
	}
	if got != nil {
		t.Errorf("ParseCondition(\"\") = %#v, want nil (no condition)", got)
	}
}

func TestEvaluate(t *testing.T) {
	values := map[string]string{
		"trackingType": "addtocart",
		"sitesearch":   "1",
	}
	get := func(field string) (string, bool) {
		v, ok := values[field]
		return v, ok
	}

	cases := []struct {
		name string
		node ConditionNode
		want bool
	}{
		{"ref set", RefNode{Field: "sitesearch"}, true},
		{"ref unset", RefNode{Field: "missing"}, false},
		{"not ref set", NotNode{Inner: RefNode{Field: "sitesearch"}}, false},
		{"eq match", EqNode{Field: "trackingType", Value: "addtocart"}, true},
		{"eq mismatch", EqNode{Field: "trackingType", Value: "event"}, false},
		{"eq unset field", EqNode{Field: "missing", Value: "x"}, false},
		{"neq match", EqNode{Field: "trackingType", Value: "event", Negate: true}, true},
		{"and both true", AndNode{Left: RefNode{Field: "sitesearch"}, Right: EqNode{Field: "trackingType", Value: "addtocart"}}, true},
		{"and one false", AndNode{Left: RefNode{Field: "sitesearch"}, Right: EqNode{Field: "trackingType", Value: "event"}}, false},
		{"or one true", OrNode{Left: RefNode{Field: "missing"}, Right: EqNode{Field: "trackingType", Value: "addtocart"}}, true},
		{"or both false", OrNode{Left: RefNode{Field: "missing"}, Right: EqNode{Field: "trackingType", Value: "event"}}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Evaluate(c.node, get); got != c.want {
				t.Errorf("Evaluate(%#v) = %v, want %v", c.node, got, c.want)
			}
		})
	}
}
