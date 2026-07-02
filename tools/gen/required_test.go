package main

import "testing"

func TestRequiredParams_known(t *testing.T) {
	got, err := RequiredParams("tag", "CustomHtml")
	if err != nil {
		t.Fatalf("RequiredParams(tag, CustomHtml) error = %v", err)
	}
	want := []string{"customHtml"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("RequiredParams(tag, CustomHtml) = %v, want %v", got, want)
	}
}

func TestRequiredParams_unknownType(t *testing.T) {
	_, err := RequiredParams("tag", "SomeBrandNewType")
	if err == nil {
		t.Fatal("RequiredParams(tag, SomeBrandNewType) error = nil, want error for unannotated type")
	}
}

func TestRequiredParams_unknownKind(t *testing.T) {
	_, err := RequiredParams("bogus-kind", "CustomHtml")
	if err == nil {
		t.Fatal("RequiredParams(bogus-kind, CustomHtml) error = nil, want error for unknown kind")
	}
}
