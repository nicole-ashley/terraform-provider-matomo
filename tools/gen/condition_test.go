package main

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
