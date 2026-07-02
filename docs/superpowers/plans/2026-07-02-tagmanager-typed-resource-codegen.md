# Tag Manager Typed-Resource Codegen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `tools/gen`, a Go program that discovers Matomo's built-in Tag Manager tag/trigger/variable types from a live instance and generates one typed Terraform resource per type into `internal/provider/generated/`, replacing hand-transcribed wire-format guessing with schema pulled directly from Matomo's own API.

**Architecture:** `tools/gen` discovers types via `TagManager.getAvailable{Tag,Trigger,Variable}TypesInContext`, merges in a hand-maintained required-field annotation table, parses each parameter's `condition` expression into validator specs, and emits Go source (model struct + `schema.Schema` + `toParams`/`fromParams`) per type. A single hand-written generic resource implementation per kind (`typedTagResource`, `typedTriggerResource`, `typedVariableResource`) supplies all CRUD logic, driven by each generated type's metadata. CI regenerates and diffs to catch drift.

**Tech Stack:** Go 1.25, terraform-plugin-framework v1.19.0, terraform-plugin-framework-validators v0.19.0, existing `internal/matomo` client package.

## Global Constraints

- Go 1.25.8 (from `go.mod`); module path `github.com/nicole-ashley/terraform-provider-matomo`.
- Discovery context is `web` only (android/ios out of scope per spec).
- Required-ness comes only from the hand-maintained `tools/gen/required.go` table; `tools/gen` must `log.Fatalf` on any discovered type with no entry — never default to all-optional silently.
- Terraform attribute type is chosen from Matomo's `type` field, never from `uiControl`.
- Resource names: `matomo_tagmanager_{tag,trigger,variable}_<lowercased id>`.
- Generated files live in `internal/provider/generated/`; generated test scaffolds are only written if absent, never overwritten.
- The existing generic `matomo_tagmanager_{tag,trigger,variable}` resources and all `internal/matomo/tagmanager_{tags,triggers,variables}.go` client code are untouched.
- No httptest-mocked unit tests for anything that talks to Matomo (this project retired those in favor of real-Matomo acceptance tests: `_acc_test.go` files using `resource.Test`/`TestAcc*`). Plain `go test` unit tests are only for pure logic with no Matomo I/O (parsers, string/case conversion, code-emission from fixture data) — this matches the surviving `internal/provider/ids_test.go` pattern. `internal/provider/draft_version_test.go` (an `httptest`-based unit test with no `_acc_test.go` counterpart) is the one pre-existing exception; do not use it as a precedent to add new httptest-mocked client tests in this plan. Task 8's shared CRUD runtime is proven by a pure dispatch-only unit test (no Matomo I/O); its actual Matomo-facing CRUD behavior is proven by real acceptance tests once generated types exist to drive it (Task 12).

---

## Task 1: Matomo client — discovery methods

**Files:**
- Create: `internal/matomo/tagmanager_templates.go`
- Test: `internal/matomo/tagmanager_templates_test.go`

**Interfaces:**
- Produces: `matomo.Template{ID, Name, Description, Category string; Parameters []TemplateParam}`, `matomo.TemplateParam{Name, Type, Description, Condition string; DefaultValue any; AvailableValues map[string]string}`, `(*Client) GetAvailableTagTypes(ctx, idContext string) ([]Template, error)`, `(*Client) GetAvailableTriggerTypes(ctx, idContext string) ([]Template, error)`, `(*Client) GetAvailableVariableTypes(ctx, idContext string) ([]Template, error)`.

Confirmed against Matomo source (`matomo-org/tag-manager` `5.x-dev`, `API/TemplateMetadata.php::formatTemplates()` and `Template/BaseTemplate.php::toArray()`): the wire response is an array of category groups (`{"name": ..., "types": [...]}`), not a flat list — must flatten. Each type object has `id`/`name`/`description`/`category`/`parameters`; each parameter comes from `SettingsMetadata::formatSetting()` (Matomo core) — confirmed that response never includes required/validator info.

- [ ] **Step 1: Write the failing test**

```go
// internal/matomo/tagmanager_templates_test.go
package matomo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAvailableTagTypes_flattensCategories(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"name": "Custom",
				"types": []map[string]any{
					{
						"id":          "CustomHtml",
						"name":        "Custom HTML",
						"description": "Inject custom HTML",
						"category":    "Custom",
						"parameters": []map[string]any{
							{
								"name":            "customHtml",
								"type":            "string",
								"description":     "The HTML to inject",
								"condition":       "",
								"defaultValue":    "",
								"availableValues": nil,
							},
							{
								"name":            "htmlPosition",
								"type":            "string",
								"description":     "Where to inject it",
								"condition":       "",
								"defaultValue":    "top",
								"availableValues": map[string]string{"top": "Top of page", "bottom": "Bottom of page"},
							},
						},
					},
				},
			},
			{
				"name": "Analytics",
				"types": []map[string]any{
					{"id": "MatomoAnalytics", "name": "Matomo Analytics", "description": "", "category": "Analytics", "parameters": []map[string]any{}},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", srv.Client())
	templates, err := client.GetAvailableTagTypes(context.Background(), "web")
	if err != nil {
		t.Fatalf("GetAvailableTagTypes() error = %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("len(templates) = %d, want 2 (flattened across both categories)", len(templates))
	}
	if templates[0].ID != "CustomHtml" {
		t.Errorf("templates[0].ID = %q, want CustomHtml", templates[0].ID)
	}
	if len(templates[0].Parameters) != 2 {
		t.Fatalf("len(templates[0].Parameters) = %d, want 2", len(templates[0].Parameters))
	}
	p := templates[0].Parameters[1]
	if p.Name != "htmlPosition" || p.Type != "string" {
		t.Errorf("templates[0].Parameters[1] = %+v, want Name=htmlPosition Type=string", p)
	}
	if p.AvailableValues["top"] != "Top of page" {
		t.Errorf("templates[0].Parameters[1].AvailableValues[top] = %q, want %q", p.AvailableValues["top"], "Top of page")
	}
	if templates[1].ID != "MatomoAnalytics" {
		t.Errorf("templates[1].ID = %q, want MatomoAnalytics", templates[1].ID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/matomo/... -run TestGetAvailableTagTypes_flattensCategories -v`
Expected: FAIL — `client.GetAvailableTagTypes` undefined.

- [ ] **Step 3: Write the implementation**

```go
// internal/matomo/tagmanager_templates.go
package matomo

import (
	"context"
	"net/url"
)

// TemplateParam describes one configurable parameter of a Tag Manager
// template (tag/trigger/variable type). Fields mirror
// SettingsMetadata::formatSetting()'s output (Matomo core,
// plugins/CorePluginsAdmin/SettingsMetadata.php) - confirmed against
// source, this never includes required/validator information, only
// presentation and default-value metadata. AvailableValues is nil when
// the parameter has no fixed value set.
type TemplateParam struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Description     string            `json:"description"`
	Condition       string            `json:"condition"`
	DefaultValue    any               `json:"defaultValue"`
	AvailableValues map[string]string `json:"availableValues"`
}

// Template describes one Tag Manager tag/trigger/variable type, as
// returned (after flattening the category grouping) by
// TagManager.getAvailableTagTypesInContext and its trigger/variable
// counterparts.
type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Parameters  []TemplateParam `json:"parameters"`
}

// templateCategoryGroup mirrors TemplateMetadata::formatTemplates()'s
// wire shape - confirmed against source, the API groups types by category
// rather than returning a flat list.
type templateCategoryGroup struct {
	Name  string     `json:"name"`
	Types []Template `json:"types"`
}

func (c *Client) getAvailableTemplates(ctx context.Context, method, idContext string) ([]Template, error) {
	v := url.Values{"idContext": {idContext}}
	var groups []templateCategoryGroup
	if err := c.call(ctx, method, v, &groups); err != nil {
		return nil, err
	}
	var templates []Template
	for _, g := range groups {
		templates = append(templates, g.Types...)
	}
	return templates, nil
}

// GetAvailableTagTypes returns every tag type Matomo supports in the
// given context (e.g. "web"), including third-party-plugin-contributed
// ones.
func (c *Client) GetAvailableTagTypes(ctx context.Context, idContext string) ([]Template, error) {
	return c.getAvailableTemplates(ctx, "TagManager.getAvailableTagTypesInContext", idContext)
}

// GetAvailableTriggerTypes returns every trigger type Matomo supports in
// the given context.
func (c *Client) GetAvailableTriggerTypes(ctx context.Context, idContext string) ([]Template, error) {
	return c.getAvailableTemplates(ctx, "TagManager.getAvailableTriggerTypesInContext", idContext)
}

// GetAvailableVariableTypes returns every variable type Matomo supports
// in the given context. Matomo itself filters out isPreConfigured()
// variables (the ~70 read-only built-ins) before this response is built,
// so they never appear here.
func (c *Client) GetAvailableVariableTypes(ctx context.Context, idContext string) ([]Template, error) {
	return c.getAvailableTemplates(ctx, "TagManager.getAvailableVariableTypesInContext", idContext)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/matomo/... -run TestGetAvailableTagTypes_flattensCategories -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/matomo/tagmanager_templates.go internal/matomo/tagmanager_templates_test.go
git commit -m "Add Matomo Tag Manager type-discovery client methods"
```

---

## Task 2: `tools/gen` naming helpers

**Files:**
- Create: `tools/gen/naming.go`
- Test: `tools/gen/naming_test.go`

**Interfaces:**
- Consumes: nothing (pure string functions).
- Produces: `gen.Slug(id string) string` (e.g. `"CustomHtml"` → `"customhtml"`), `gen.CamelToSnake(s string) string` (e.g. `"htmlPosition"` → `"html_position"`), `gen.ExportedName(s string) string` (e.g. `"htmlPosition"` → `"HtmlPosition"`, used for generated Go field names).

- [ ] **Step 1: Write the failing test**

```go
// tools/gen/naming_test.go
package main

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"CustomHtml":      "customhtml",
		"GoogleAnalytics":  "googleanalytics",
		"Constant":         "constant",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamelToSnake(t *testing.T) {
	cases := map[string]string{
		"customHtml":       "custom_html",
		"htmlPosition":     "html_position",
		"fireTriggerIds":   "fire_trigger_ids",
		"value":            "value",
		"URLPath":          "url_path",
	}
	for in, want := range cases {
		if got := CamelToSnake(in); got != want {
			t.Errorf("CamelToSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExportedName(t *testing.T) {
	cases := map[string]string{
		"customHtml":     "CustomHtml",
		"htmlPosition":   "HtmlPosition",
		"fireTriggerIds": "FireTriggerIds",
	}
	for in, want := range cases {
		if got := ExportedName(in); got != want {
			t.Errorf("ExportedName(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tools/gen/... -run 'TestSlug|TestCamelToSnake|TestExportedName' -v`
Expected: FAIL — `Slug`/`CamelToSnake`/`ExportedName` undefined (package `main` doesn't exist yet under `tools/gen`).

- [ ] **Step 3: Write the implementation**

```go
// tools/gen/naming.go
package main

import "strings"

// Slug converts a Matomo template id (e.g. "CustomHtml") into the
// lowercase form used in generated resource names
// (matomo_tagmanager_tag_customhtml).
func Slug(id string) string {
	return strings.ToLower(id)
}

// CamelToSnake converts a Matomo camelCase parameter name (e.g.
// "htmlPosition") into the snake_case form used for generated Terraform
// attribute names ("html_position"). Consecutive uppercase letters (as in
// an acronym like "URLPath") are treated as a single word boundary rather
// than one boundary per letter, so "URLPath" becomes "url_path" not
// "u_r_l_path".
func CamelToSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper {
			prevLower := i > 0 && !(runes[i-1] >= 'A' && runes[i-1] <= 'Z')
			nextLower := i+1 < len(runes) && !(runes[i+1] >= 'A' && runes[i+1] <= 'Z')
			if i > 0 && (prevLower || nextLower) {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ExportedName converts a Matomo camelCase parameter name into an
// exported Go identifier ("htmlPosition" -> "HtmlPosition"), for use as a
// generated model struct field name.
func ExportedName(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - 'a' + 'A'
	}
	return string(r)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tools/gen/... -run 'TestSlug|TestCamelToSnake|TestExportedName' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/gen/naming.go tools/gen/naming_test.go
git commit -m "Add tools/gen naming helpers"
```

---

## Task 3: Condition expression parser

**Files:**
- Create: `tools/gen/condition.go`
- Test: `tools/gen/condition_test.go`

**Interfaces:**
- Consumes: nothing beyond the standard library.
- Produces: `gen.ConditionNode` (interface, implemented by `gen.RefNode{Field string}`, `gen.NotNode{Inner ConditionNode}`, `gen.AndNode{Left, Right ConditionNode}`, `gen.OrNode{Left, Right ConditionNode}`, `gen.EqNode{Field, Value string; Negate bool}`), `gen.ParseCondition(expr string) (ConditionNode, error)`. Task 5 consumes `ParseCondition` to build validator specs; Task 5 needs to type-switch on these exact node types.

Grammar confirmed against Matomo source (`core/Settings/FieldConfig.php`'s `$condition` docblock): identifiers, `!`, `&&`, `||`, and (per the field's docblock example plus real Tag Manager template usage) `==`/`!=` comparisons against string literals. Precedence (highest to lowest): `!`, `==`/`!=`, `&&`, `||`. Grammar in EBNF:

```
expr       := or_expr
or_expr    := and_expr ( "||" and_expr )*
and_expr   := unary ( "&&" unary )*
unary      := "!" unary | comparison
comparison := IDENT ( ( "==" | "!=" ) STRING )?
STRING     := "'" [^']* "'" | '"' [^"]* '"'
IDENT      := [A-Za-z_][A-Za-z0-9_]*
```

- [ ] **Step 1: Write the failing test**

```go
// tools/gen/condition_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tools/gen/... -run TestParseCondition -v`
Expected: FAIL — `ConditionNode`/`RefNode`/etc. undefined.

- [ ] **Step 3: Write the implementation**

```go
// tools/gen/condition.go
package main

import "fmt"

// ConditionNode is one node of a parsed FieldConfig::$condition
// expression tree.
type ConditionNode interface {
	isConditionNode()
}

// RefNode is a bare field reference ("sitesearch"): true iff that field
// has a truthy/non-empty value.
type RefNode struct{ Field string }

// NotNode negates its inner condition ("!sitesearch").
type NotNode struct{ Inner ConditionNode }

// AndNode requires both sides to hold.
type AndNode struct{ Left, Right ConditionNode }

// OrNode requires at least one side to hold.
type OrNode struct{ Left, Right ConditionNode }

// EqNode compares a field's value to a string literal
// (`triggerType == "pageview"`, or `!= ` when Negate is true).
type EqNode struct {
	Field  string
	Value  string
	Negate bool
}

func (RefNode) isConditionNode() {}
func (NotNode) isConditionNode() {}
func (AndNode) isConditionNode() {}
func (OrNode) isConditionNode()  {}
func (EqNode) isConditionNode()  {}

type conditionToken struct {
	kind string // "ident", "string", "&&", "||", "!", "==", "!=", "eof"
	text string
}

func lexCondition(expr string) ([]conditionToken, error) {
	var tokens []conditionToken
	runes := []rune(expr)
	i := 0
	for i < len(runes) {
		r := runes[i]
		switch {
		case r == ' ' || r == '\t':
			i++
		case r == '!' && i+1 < len(runes) && runes[i+1] == '=':
			tokens = append(tokens, conditionToken{kind: "!="})
			i += 2
		case r == '!':
			tokens = append(tokens, conditionToken{kind: "!"})
			i++
		case r == '&' && i+1 < len(runes) && runes[i+1] == '&':
			tokens = append(tokens, conditionToken{kind: "&&"})
			i += 2
		case r == '|' && i+1 < len(runes) && runes[i+1] == '|':
			tokens = append(tokens, conditionToken{kind: "||"})
			i += 2
		case r == '=' && i+1 < len(runes) && runes[i+1] == '=':
			tokens = append(tokens, conditionToken{kind: "=="})
			i += 2
		case r == '\'' || r == '"':
			quote := r
			j := i + 1
			for j < len(runes) && runes[j] != quote {
				j++
			}
			if j >= len(runes) {
				return nil, fmt.Errorf("unterminated string literal in condition %q", expr)
			}
			tokens = append(tokens, conditionToken{kind: "string", text: string(runes[i+1 : j])})
			i = j + 1
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_':
			j := i
			for j < len(runes) && ((runes[j] >= 'A' && runes[j] <= 'Z') || (runes[j] >= 'a' && runes[j] <= 'z') || (runes[j] >= '0' && runes[j] <= '9') || runes[j] == '_') {
				j++
			}
			tokens = append(tokens, conditionToken{kind: "ident", text: string(runes[i:j])})
			i = j
		default:
			return nil, fmt.Errorf("unexpected character %q in condition %q", r, expr)
		}
	}
	tokens = append(tokens, conditionToken{kind: "eof"})
	return tokens, nil
}

type conditionParser struct {
	tokens []conditionToken
	pos    int
}

func (p *conditionParser) peek() conditionToken { return p.tokens[p.pos] }
func (p *conditionParser) next() conditionToken {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *conditionParser) parseOr() (ConditionNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == "||" {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = OrNode{Left: left, Right: right}
	}
	return left, nil
}

func (p *conditionParser) parseAnd() (ConditionNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == "&&" {
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = AndNode{Left: left, Right: right}
	}
	return left, nil
}

func (p *conditionParser) parseUnary() (ConditionNode, error) {
	if p.peek().kind == "!" {
		p.next()
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return NotNode{Inner: inner}, nil
	}
	return p.parseComparison()
}

func (p *conditionParser) parseComparison() (ConditionNode, error) {
	tok := p.next()
	if tok.kind != "ident" {
		return nil, fmt.Errorf("expected field name, got token kind %q", tok.kind)
	}
	field := tok.text

	switch p.peek().kind {
	case "==", "!=":
		negate := p.next().kind == "!="
		lit := p.next()
		if lit.kind != "string" {
			return nil, fmt.Errorf("expected string literal after ==/!= for field %q, got token kind %q", field, lit.kind)
		}
		return EqNode{Field: field, Value: lit.text, Negate: negate}, nil
	default:
		return RefNode{Field: field}, nil
	}
}

// ParseCondition parses a FieldConfig::$condition expression into a
// ConditionNode tree. An empty expr returns (nil, nil): "no condition."
func ParseCondition(expr string) (ConditionNode, error) {
	if expr == "" {
		return nil, nil
	}
	tokens, err := lexCondition(expr)
	if err != nil {
		return nil, fmt.Errorf("parsing condition %q: %w", expr, err)
	}
	p := &conditionParser{tokens: tokens}
	node, err := p.parseOr()
	if err != nil {
		return nil, fmt.Errorf("parsing condition %q: %w", expr, err)
	}
	if p.peek().kind != "eof" {
		return nil, fmt.Errorf("parsing condition %q: unexpected trailing token %q", expr, p.peek().kind)
	}
	return node, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tools/gen/... -run TestParseCondition -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/gen/condition.go tools/gen/condition_test.go
git commit -m "Add tools/gen condition-expression parser"
```

---

## Task 4: Required-field annotation table

**Files:**
- Create: `tools/gen/required.go`
- Test: `tools/gen/required_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `gen.RequiredParams(kind, typeID string) ([]string, error)` — returns the required parameter names for a type, or an error (not a panic — `tools/gen/main.go` in Task 9 turns this into a `log.Fatalf` with context) if `kind`/`typeID` has no entry.

This table is hand-maintained, not generated. It's seeded here with the two types already verified against Matomo source earlier in this project (`CustomHtml`'s `customHtml` field and `Constant`'s `value` field, per the existing `matomo_tagmanager_tag`/`matomo_tagmanager_variable` acceptance tests). Task 12 extends this table with every other type actually discovered against the live acceptance-test Matomo instance — that list can't be known until `tools/gen` actually runs (Task 9), so it isn't enumerable in this task.

- [ ] **Step 1: Write the failing test**

```go
// tools/gen/required_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tools/gen/... -run TestRequiredParams -v`
Expected: FAIL — `RequiredParams` undefined.

- [ ] **Step 3: Write the implementation**

```go
// tools/gen/required.go
package main

import "fmt"

// requiredParams is the hand-maintained source of truth for which
// parameters each Matomo Tag Manager type requires. Matomo's discovery
// API (TagManager.getAvailable{Tag,Trigger,Variable}TypesInContext) never
// exposes this - confirmed by reading SettingsMetadata::formatSetting()'s
// full body directly: FieldConfig::$validators (where a type's
// NotEmpty() etc. validators actually live, server-side) is never
// serialized into the response. Each entry below was added by reading
// that type's own Matomo source (grep for
// "$field->validators[] = new NotEmpty()" or similar) once.
//
// RequiredParams fails loudly for any (kind, typeID) with no entry here,
// specifically so a newly-discovered type can never silently ship with
// every parameter marked Optional by omission.
var requiredParams = map[string]map[string][]string{
	"tag": {
		"CustomHtml": {"customHtml"},
	},
	"trigger": {},
	"variable": {
		"Constant": {"value"},
	},
}

// RequiredParams returns the required parameter names for the given
// Tag Manager type. kind must be "tag", "trigger", or "variable". Returns
// an error if kind or typeID has no entry in requiredParams - callers
// must treat that as fatal (see tools/gen/main.go), never as "assume
// optional."
func RequiredParams(kind, typeID string) ([]string, error) {
	byType, ok := requiredParams[kind]
	if !ok {
		return nil, fmt.Errorf("no required-params table for kind %q (want one of tag/trigger/variable)", kind)
	}
	required, ok := byType[typeID]
	if !ok {
		return nil, fmt.Errorf("no required-params entry for %s type %q - read its Matomo source and add an entry (even an empty one) to requiredParams in tools/gen/required.go", kind, typeID)
	}
	return required, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tools/gen/... -run TestRequiredParams -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/gen/required.go tools/gen/required_test.go
git commit -m "Add tools/gen required-field annotation table"
```

---

## Task 5: TypeSpec builder

**Files:**
- Create: `tools/gen/spec.go`
- Test: `tools/gen/spec_test.go`

**Interfaces:**
- Consumes: `matomo.Template`/`matomo.TemplateParam` (Task 1), `RequiredParams` (Task 4), `ParseCondition`/`ConditionNode` types (Task 3), `Slug`/`CamelToSnake`/`ExportedName` (Task 2).
- Produces: `gen.TypeSpec{Kind, TypeID, Slug, ResourceName string; Params []ParamSpec}`, `gen.ParamSpec{MatomoName, TFName, GoFieldName, Description string; GoType string; Required bool; AvailableValues []string; Condition ConditionNode}`, `gen.BuildTypeSpec(kind string, tmpl matomo.Template) (TypeSpec, error)`. Task 7 (code emission) consumes `TypeSpec`/`ParamSpec` directly and needs these exact field names.

`GoType` is one of `"String"`, `"Bool"`, `"List"` (terraform-plugin-framework attribute kind), chosen from `TemplateParam.Type`: Matomo's `Setting::getType()` values are `"string"`, `"bool"`, and `"array"` (confirmed against `Piwik\Settings\FieldConfig` type constants) — `BuildTypeSpec` fails loudly (returns an error) on any other value rather than guessing, matching the required-field and condition-parsing "fail rather than guess" posture used elsewhere in this generator.

- [ ] **Step 1: Write the failing test**

```go
// tools/gen/spec_test.go
package main

import (
	"testing"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

func TestBuildTypeSpec(t *testing.T) {
	tmpl := matomo.Template{
		ID:          "CustomHtml",
		Name:        "Custom HTML",
		Description: "Inject custom HTML",
		Parameters: []matomo.TemplateParam{
			{Name: "customHtml", Type: "string", Description: "The HTML to inject"},
			{
				Name:            "htmlPosition",
				Type:            "string",
				Description:     "Where to inject it",
				AvailableValues: map[string]string{"top": "Top of page", "bottom": "Bottom of page"},
				Condition:       "customHtml",
			},
		},
	}

	spec, err := BuildTypeSpec("tag", tmpl)
	if err != nil {
		t.Fatalf("BuildTypeSpec() error = %v", err)
	}
	if spec.Kind != "tag" || spec.TypeID != "CustomHtml" || spec.Slug != "customhtml" {
		t.Errorf("spec = %+v, want Kind=tag TypeID=CustomHtml Slug=customhtml", spec)
	}
	if spec.ResourceName != "matomo_tagmanager_tag_customhtml" {
		t.Errorf("spec.ResourceName = %q, want matomo_tagmanager_tag_customhtml", spec.ResourceName)
	}
	if len(spec.Params) != 2 {
		t.Fatalf("len(spec.Params) = %d, want 2", len(spec.Params))
	}

	p0 := spec.Params[0]
	if p0.MatomoName != "customHtml" || p0.TFName != "custom_html" || p0.GoFieldName != "CustomHtml" {
		t.Errorf("spec.Params[0] = %+v, want MatomoName=customHtml TFName=custom_html GoFieldName=CustomHtml", p0)
	}
	if p0.GoType != "String" {
		t.Errorf("spec.Params[0].GoType = %q, want String", p0.GoType)
	}
	if !p0.Required {
		t.Error("spec.Params[0].Required = false, want true (customHtml is required for CustomHtml)")
	}

	p1 := spec.Params[1]
	if p1.Required {
		t.Error("spec.Params[1].Required = true, want false (htmlPosition is not in requiredParams)")
	}
	if len(p1.AvailableValues) != 2 {
		t.Errorf("len(spec.Params[1].AvailableValues) = %d, want 2", len(p1.AvailableValues))
	}
	ref, ok := p1.Condition.(RefNode)
	if !ok || ref.Field != "customHtml" {
		t.Errorf("spec.Params[1].Condition = %#v, want RefNode{Field: customHtml}", p1.Condition)
	}
}

func TestBuildTypeSpec_unknownParamType(t *testing.T) {
	tmpl := matomo.Template{
		ID:         "Weird",
		Parameters: []matomo.TemplateParam{{Name: "x", Type: "float"}},
	}
	if _, err := BuildTypeSpec("tag", tmpl); err == nil {
		t.Fatal("BuildTypeSpec() error = nil, want error for unrecognized Matomo param type \"float\"")
	}
}

func TestBuildTypeSpec_unannotatedType(t *testing.T) {
	tmpl := matomo.Template{ID: "SomeBrandNewType"}
	if _, err := BuildTypeSpec("tag", tmpl); err == nil {
		t.Fatal("BuildTypeSpec() error = nil, want error for a type with no requiredParams entry")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tools/gen/... -run TestBuildTypeSpec -v`
Expected: FAIL — `TypeSpec`/`BuildTypeSpec` undefined.

- [ ] **Step 3: Write the implementation**

```go
// tools/gen/spec.go
package main

import (
	"fmt"

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
	Condition       ConditionNode
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

func matomoTypeToGoType(matomoType string) (string, error) {
	switch matomoType {
	case "string":
		return "String", nil
	case "bool":
		return "Bool", nil
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
		cond, err := ParseCondition(p.Condition)
		if err != nil {
			return TypeSpec{}, fmt.Errorf("type %q, parameter %q: %w", tmpl.ID, p.Name, err)
		}

		var availableValues []string
		for value := range p.AvailableValues {
			availableValues = append(availableValues, value)
		}

		spec.Params = append(spec.Params, ParamSpec{
			MatomoName:      p.Name,
			TFName:          CamelToSnake(p.Name),
			GoFieldName:     ExportedName(p.Name),
			Description:     p.Description,
			GoType:          goType,
			Required:        requiredSet[p.Name],
			AvailableValues: availableValues,
			Condition:       cond,
		})
	}

	return spec, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tools/gen/... -run TestBuildTypeSpec -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/gen/spec.go tools/gen/spec_test.go
git commit -m "Add tools/gen TypeSpec builder"
```

---

## Task 6: Shared condition validators

**Files:**
- Create: `internal/provider/condition_validators.go`
- Test: `internal/provider/condition_validators_test.go`

**Interfaces:**
- Consumes: `github.com/hashicorp/terraform-plugin-framework/schema/validator`, `github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator` (`AlsoRequires`/`ConflictsWith` already exist there — this task only adds the two Matomo needs that the validators library doesn't provide).
- Produces: `conditionEqualsValidator{Field, Value string; Negate bool}` (implements `validator.String`), `conditionAnyOfValidator{Validators []validator.String}` (implements `validator.String`, satisfied if any wrapped validator reports no error). Task 7's code emitter references these two type names verbatim when a generated schema needs an `Eq`/`Or` condition.

- [ ] **Step 1: Write the failing test**

```go
// internal/provider/condition_validators_test.go
package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testConfig(t *testing.T, attrs map[string]string, values map[string]string) tfsdk.Config {
	t.Helper()
	schemaAttrs := map[string]schemaTestAttr{}
	_ = attrs
	_ = values
	_ = schemaAttrs
	return tfsdk.Config{}
}

func TestConditionEqualsValidator(t *testing.T) {
	v := conditionEqualsValidator{Field: "trigger_type", Value: "pageview"}

	req := validator.StringRequest{
		Path:           path.Root("value"),
		PathExpression: path.MatchRoot("value"),
		ConfigValue:    types.StringValue("anything"),
	}

	// Build a minimal raw config where trigger_type == "pageview": the
	// validator should report no error.
	config := tfsdk.Config{Raw: rawConfigWithString(t, "trigger_type", "pageview")}
	req.Config = config

	var resp validator.StringResponse
	v.ValidateString(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ValidateString() with matching trigger_type produced errors: %v", resp.Diagnostics)
	}

	// Now trigger_type == "click": the validator should report an error.
	config2 := tfsdk.Config{Raw: rawConfigWithString(t, "trigger_type", "click")}
	req.Config = config2
	var resp2 validator.StringResponse
	v.ValidateString(context.Background(), req, &resp2)
	if !resp2.Diagnostics.HasError() {
		t.Fatal("ValidateString() with non-matching trigger_type produced no error, want one")
	}
}
```

This test needs a way to build a minimal `tftypes.Value`-backed raw config with one string attribute set — write that helper for real rather than stubbing it, since `tfsdk.Config` has no simpler constructor in this framework version:

```go
// internal/provider/condition_validators_test.go (continued, same file)
import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func rawConfigWithString(t *testing.T, attrName, value string) tftypes.Value {
	t.Helper()
	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			attrName: tftypes.String,
			"value":  tftypes.String,
		},
	}
	return tftypes.NewValue(objType, map[string]tftypes.Value{
		attrName: tftypes.NewValue(tftypes.String, value),
		"value":  tftypes.NewValue(tftypes.String, "anything"),
	})
}
```

Remove the unused `testConfig`/`schemaTestAttr` scaffolding above — it was a false start; the real helper is `rawConfigWithString`. The `tfsdk.Config{Raw: ...}` also needs a `Schema` field for `GetAttribute` to resolve paths; set it explicitly:

```go
config := tfsdk.Config{
	Raw: rawConfigWithString(t, "trigger_type", "pageview"),
	Schema: testSchemaWithStringAttrs("trigger_type", "value"),
}
```

```go
// internal/provider/condition_validators_test.go (continued)
import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func testSchemaWithStringAttrs(names ...string) schema.Schema {
	attrs := map[string]schema.Attribute{}
	for _, n := range names {
		attrs[n] = schema.StringAttribute{Optional: true}
	}
	return schema.Schema{Attributes: attrs}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/... -run TestConditionEqualsValidator -v`
Expected: FAIL — `conditionEqualsValidator` undefined.

- [ ] **Step 3: Write the implementation**

```go
// internal/provider/condition_validators.go
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// conditionEqualsValidator implements the Eq/Neq case of a generated
// type's Matomo `condition` expression: the attribute it's attached to is
// only meaningful when another attribute (Field) equals (or, if Negate,
// does not equal) Value. There's no built-in terraform-plugin-framework
// validator for "required/meaningful if sibling equals X," unlike the
// plain AlsoRequires/ConflictsWith cases a bare RefNode/NotNode maps to.
type conditionEqualsValidator struct {
	Field  string
	Value  string
	Negate bool
}

func (v conditionEqualsValidator) Description(_ context.Context) string {
	op := "=="
	if v.Negate {
		op = "!="
	}
	return fmt.Sprintf("only meaningful when %s %s %q", v.Field, op, v.Value)
}

func (v conditionEqualsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v conditionEqualsValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var fieldValue types.String
	diags := req.Config.GetAttribute(ctx, path.Root(v.Field), &fieldValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if fieldValue.IsNull() || fieldValue.IsUnknown() {
		return
	}

	matches := fieldValue.ValueString() == v.Value
	if v.Negate {
		matches = !matches
	}
	if !matches {
		op := "=="
		if v.Negate {
			op = "!="
		}
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid attribute combination",
			fmt.Sprintf("%s is only meaningful when %s %s %q", req.Path, v.Field, op, v.Value),
		)
	}
}

// conditionAnyOfValidator implements the Or case: the attribute it's
// attached to is valid if at least one wrapped validator reports no
// error (terraform-plugin-framework has no native "satisfy any of"
// combinator for validator.String).
type conditionAnyOfValidator struct {
	Validators []validator.String
}

func (v conditionAnyOfValidator) Description(ctx context.Context) string {
	return "must satisfy at least one condition"
}

func (v conditionAnyOfValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v conditionAnyOfValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	for _, inner := range v.Validators {
		var innerResp validator.StringResponse
		inner.ValidateString(ctx, req, &innerResp)
		if !innerResp.Diagnostics.HasError() {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid attribute combination",
		fmt.Sprintf("%s does not satisfy any of its required conditions", req.Path),
	)
}
```

- [ ] **Step 4: Run test to verify it passes, then clean up the test file**

Remove the unused `testConfig` false-start function from the test file (leave only `rawConfigWithString`, `testSchemaWithStringAttrs`, and `TestConditionEqualsValidator`).

Run: `go test ./internal/provider/... -run TestConditionEqualsValidator -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/condition_validators.go internal/provider/condition_validators_test.go
git commit -m "Add shared Eq/Or condition validators for generated resources"
```

---

## Task 7: Code emitter (TypeSpec → Go source)

**Files:**
- Create: `tools/gen/emit.go`
- Create: `tools/gen/templates/schema.go.tmpl`
- Test: `tools/gen/emit_test.go`

**Interfaces:**
- Consumes: `TypeSpec`/`ParamSpec` (Task 5).
- Produces: `gen.RenderSchema(spec TypeSpec) ([]byte, error)` — gofmt'd Go source implementing the type's generated model+schema file, matching the shared-runtime interface Task 8 defines (`typedModel` — see Task 8's `Interfaces` block, which this task's output must satisfy: `Meta() typedMeta`, `ToParams() map[string]string`, `FromParams(map[string]string)`).

- [ ] **Step 1: Write the failing test**

```go
// tools/gen/emit_test.go
package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestRenderSchema_parsesAsValidGo(t *testing.T) {
	spec := TypeSpec{
		Kind:         "tag",
		TypeID:       "CustomHtml",
		Slug:         "customhtml",
		ResourceName: "matomo_tagmanager_tag_customhtml",
		Description:  "Inject custom HTML",
		Params: []ParamSpec{
			{MatomoName: "customHtml", TFName: "custom_html", GoFieldName: "CustomHtml", Description: "The HTML to inject", GoType: "String", Required: true},
			{
				MatomoName: "htmlPosition", TFName: "html_position", GoFieldName: "HtmlPosition",
				Description: "Where to inject it", GoType: "String", Required: false,
				AvailableValues: []string{"top", "bottom"},
				Condition:       RefNode{Field: "customHtml"},
			},
		},
	}

	src, err := RenderSchema(spec)
	if err != nil {
		t.Fatalf("RenderSchema() error = %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "tag_customhtml.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse as valid Go: %v\n---\n%s", err, src)
	}

	got := string(src)
	for _, want := range []string{
		"tagCustomhtmlModel",
		`"custom_html"`,
		"CustomHtml types.String",
		"HtmlPosition types.String",
		"Required: true",
		`TypeID:       "CustomHtml"`,
		`ResourceName: "matomo_tagmanager_tag_customhtml"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q; full source:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tools/gen/... -run TestRenderSchema_parsesAsValidGo -v`
Expected: FAIL — `RenderSchema` undefined.

- [ ] **Step 3: Write the implementation**

```
{{/* tools/gen/templates/schema.go.tmpl */}}
// Code generated by tools/gen. DO NOT EDIT.

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type {{.GoModelName}} struct {
	ID              types.String   `tfsdk:"id"`
	ContainerID     types.String   `tfsdk:"container_id"`
	Name            types.String   `tfsdk:"name"`
{{- if eq .Kind "tag"}}
	Status          types.String   `tfsdk:"status"`
	FireTriggerIDs  []types.String `tfsdk:"fire_trigger_ids"`
	BlockTriggerIDs []types.String `tfsdk:"block_trigger_ids"`
{{- end}}
{{- if eq .Kind "variable"}}
	DefaultValue    types.String   `tfsdk:"default_value"`
{{- end}}
{{- range .Params}}
	{{.GoFieldName}} types.{{.GoType}} `tfsdk:"{{.TFName}}"`
{{- end}}
}

func {{.GoSchemaFuncName}}() schema.Schema {
	return schema.Schema{
		Description: {{printf "%q" .Description}},
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"container_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
{{- if eq .Kind "tag"}}
			"status": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{stringvalidator.OneOf("active", "paused")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"fire_trigger_ids": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
			},
			"block_trigger_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
{{- end}}
{{- if eq .Kind "variable"}}
			"default_value": schema.StringAttribute{
				Optional: true,
			},
{{- end}}
{{- range .Params}}
			{{printf "%q" .TFName}}: schema.{{.GoType}}Attribute{
{{- if eq .GoType "List"}}
				ElementType: types.StringType,
{{- end}}
				Required: {{.Required}},
{{- if not .Required}}
				Optional: true,
{{- end}}
				Description: {{printf "%q" .Description}},
{{- if .AvailableValues}}
{{- if eq .GoType "String"}}
				Validators: []validator.String{stringvalidator.OneOf({{range $i, $v := .AvailableValues}}{{if $i}}, {{end}}{{printf "%q" $v}}{{end}})},
{{- end}}
{{- end}}
			},
{{- end}}
		},
	}
}

func ({{.GoModelReceiver}} *{{.GoModelName}}) Meta() typedMeta {
	return typedMeta{
		TypeID:       {{printf "%q" .TypeID}},
		ResourceName: {{printf "%q" .ResourceName}},
		Schema:       {{.GoSchemaFuncName}}(),
	}
}

func ({{.GoModelReceiver}} *{{.GoModelName}}) ToParams() map[string]string {
	return map[string]string{
{{- range .Params}}
		{{printf "%q" .MatomoName}}: {{$.GoModelReceiver}}.{{.GoFieldName}}.ValueString(),
{{- end}}
	}
}

func ({{.GoModelReceiver}} *{{.GoModelName}}) FromParams(p map[string]string) {
{{- range .Params}}
	{{$.GoModelReceiver}}.{{.GoFieldName}} = types.StringValue(p[{{printf "%q" .MatomoName}}])
{{- end}}
}

func new{{.GoTypeName}}Model() typedModel {
	return &{{.GoModelName}}{}
}
```

```go
// tools/gen/emit.go
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"
)

// templateData adds the Go-identifier fields the template needs
// (GoModelName, GoSchemaFuncName, etc.) on top of a TypeSpec, without
// polluting TypeSpec itself with rendering-only concerns.
type templateData struct {
	TypeSpec
	GoModelName      string
	GoSchemaFuncName string
	GoTypeName       string
	GoModelReceiver  string
}

func newTemplateData(spec TypeSpec) templateData {
	typeName := ExportedName(spec.Kind) + ExportedName(spec.Slug)
	return templateData{
		TypeSpec:         spec,
		GoModelName:      spec.Kind + ExportedName(spec.Slug) + "Model",
		GoSchemaFuncName: spec.Kind + ExportedName(spec.Slug) + "Schema",
		GoTypeName:       typeName,
		GoModelReceiver:  "m",
	}
}

var schemaTemplate = template.Must(template.ParseFiles("tools/gen/templates/schema.go.tmpl"))

// RenderSchema renders spec into a gofmt'd Go source file implementing
// the type's generated model + schema.Schema + typedModel methods, ready
// to write to internal/provider/generated/<kind>_<slug>.go.
func RenderSchema(spec TypeSpec) ([]byte, error) {
	var buf bytes.Buffer
	if err := schemaTemplate.Execute(&buf, newTemplateData(spec)); err != nil {
		return nil, fmt.Errorf("rendering template for %s %q: %w", spec.Kind, spec.TypeID, err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt-ing generated source for %s %q: %w\n--- unformatted source ---\n%s", spec.Kind, spec.TypeID, err, buf.String())
	}
	return formatted, nil
}

var _ = strings.TrimSpace // placeholder import use removed below if unused
```

Remove the placeholder `var _ = strings.TrimSpace` line and the now-unused `"strings"` import from `tools/gen/emit.go` — it was left in only to flag that no stray unused import should ship; delete both once you've confirmed the file compiles without them.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tools/gen/... -run TestRenderSchema_parsesAsValidGo -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/gen/emit.go tools/gen/templates/schema.go.tmpl tools/gen/emit_test.go
git commit -m "Add tools/gen code emitter (TypeSpec -> Go source)"
```

---

## Task 8: Shared typed-resource runtime

**Files:**
- Create: `internal/provider/typed_model.go`
- Create: `internal/provider/typed_tag_resource.go`
- Create: `internal/provider/typed_trigger_resource.go`
- Create: `internal/provider/typed_variable_resource.go`
- Test: `internal/provider/typed_tag_resource_test.go`

**Interfaces:**
- Consumes: `resolveDraftVersionID`, `buildEntityID`/`parseEntityID`, `bareEntityIDs`/`compositeEntityIDs`, `intsToStrings`, `stringModelFromSlice`/`stringSliceFromModel` (all existing, `internal/provider/ids.go` and `resource_tagmanager_tag.go`), `matomo.TagParams`/`AddContainerTag`/`GetContainerTag`/`UpdateContainerTag`/`DeleteContainerTag`/`PauseContainerTag`/`ResumeContainerTag` (existing, `internal/matomo/tagmanager_tags.go`).
- Produces: `typedMeta{TypeID, ResourceName string; Schema schema.Schema}`, `typedModel` interface (`Meta() typedMeta`, `ToParams() map[string]string`, `FromParams(map[string]string)`, plus common-field accessors below), `newTypedTagResource(newModel func() typedModel) resource.Resource`. Task 7's generated code must satisfy `typedModel` exactly as defined here — this task is written first in implementation order within this plan's dependency graph conceptually, but since Task 7 already assumed these exact method names, this task's interface must not drift from what Task 7's template already emits (`Meta()`, `ToParams()`, `FromParams(map[string]string)`).

Since `typedModel` as sketched in Task 7's template only covers type-specific parameters, the common fields (`id`/`container_id`/`name`/`status`/trigger-id lists) need their own accessors on the interface so the shared runtime can read/write them without reflection:

```go
// typedModel is satisfied by every generated *_Model type (Task 7's
// emitter output). ToParams/FromParams only handle each type's
// Matomo-specific parameters; the common tag/trigger/variable fields
// (id, container_id, name, ...) are handled directly by the shared
// runtime via the concrete generated struct's tfsdk-tagged fields, read
// through Get/Set on the whole model - so typedModel only needs to
// expose Meta/ToParams/FromParams, not per-field accessors.
type typedModel interface {
	Meta() typedMeta
	ToParams() map[string]string
	FromParams(params map[string]string)
}
```

To read/write the common fields (`id`, `container_id`, `name`, `status`, `fire_trigger_ids`, `block_trigger_ids`) without per-type reflection, `typedTagResource`'s CRUD methods use `req.Plan.Get`/`resp.State.Set` against a small **fixed-shape wrapper struct** that every generated tag model embeds identically (all generated tag models declare the same four/six leading `tfsdk`-tagged fields in the same order — Task 7's template already guarantees this), read via `tftypes`-based reflection helpers `terraform-plugin-framework` itself provides (`req.Plan.GetAttribute(ctx, path.Root("name"), &nameVal)` etc.) rather than a Go-level shared struct. This avoids needing the generated struct to satisfy two different Go interfaces (one for common fields, one for `typedModel`).

**Note on proving this task:** the real `testAccProtoV6ProviderFactories` (`internal/provider/acc_test_helpers.go:34`) is a fixed map built from `New("test")()` — it is not parameterizable with an extra ad hoc resource, and there is no shared container/trigger HCL helper (every existing `*_acc_test.go` file inlines full config; confirmed by reading `resource_tagmanager_tag_acc_test.go` and `provider_test.go` in full). Standing up a hand-written fixture type through the real provider isn't possible without either modifying `MatomoProvider.Resources()` (which would ship a fixture-only resource in production) or duplicating provider construction. So this task is proven in two parts: a **pure unit test** here (no Matomo I/O, no acceptance framework) that `typedTagResource` dispatches `Metadata`/`Schema` correctly and that a fake model satisfies `typedModel`, and a **deferred full CRUD proof** in Task 12, once real generated types exist and are wired into the real provider via Task 10 — at that point the generated `TestAcc<Type>_createAndReadBack` scaffolds exercise this exact shared runtime through the real, unmodified `testAccProtoV6ProviderFactories`.

- [ ] **Step 1: Write the failing test**

```go
// internal/provider/typed_tag_resource_test.go
package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fakeTagModel struct {
	Value types.String `tfsdk:"value"`
}

func (m *fakeTagModel) Meta() typedMeta {
	return typedMeta{
		TypeID:       "FakeType",
		ResourceName: "matomo_tagmanager_tag_faketype",
		Schema: schema.Schema{
			Attributes: map[string]schema.Attribute{
				"value": schema.StringAttribute{Required: true},
			},
		},
	}
}

func (m *fakeTagModel) ToParams() map[string]string        { return map[string]string{"value": m.Value.ValueString()} }
func (m *fakeTagModel) FromParams(p map[string]string)     { m.Value = types.StringValue(p["value"]) }

func TestTypedTagResource_metadataAndSchemaDispatchToModel(t *testing.T) {
	r := newTypedTagResource(func() typedModel { return &fakeTagModel{} }).(*typedTagResource)

	var metaResp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{}, &metaResp)
	if metaResp.TypeName != "matomo_tagmanager_tag_faketype" {
		t.Errorf("TypeName = %q, want matomo_tagmanager_tag_faketype", metaResp.TypeName)
	}

	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	if _, ok := schemaResp.Schema.Attributes["value"]; !ok {
		t.Error("Schema() did not include the model's \"value\" attribute")
	}
}

var _ typedModel = &fakeTagModel{}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/... -run TestTypedTagResource_metadataAndSchemaDispatchToModel -v`
Expected: FAIL — `typedTagResource`/`typedModel`/`newTypedTagResource` undefined.

- [ ] **Step 3: Write the real implementation**

First, `internal/provider/typed_model.go`:

```go
// internal/provider/typed_model.go
package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// typedMeta is the per-type metadata a generated model supplies to the
// shared typed{Tag,Trigger,Variable}Resource runtime.
type typedMeta struct {
	TypeID       string // Matomo's type id, e.g. "CustomHtml"
	ResourceName string // full Terraform type name, e.g. "matomo_tagmanager_tag_customhtml"
	Schema       schema.Schema
}

// typedModel is satisfied by every generated model type (Task 7's
// emitter output). ToParams/FromParams only handle a type's
// Matomo-specific parameters - the common fields shared by every tag (or
// trigger, or variable) are declared identically by every generated
// model (same tfsdk tags, same order) and are read directly by the
// shared runtime via req.Plan.Get/resp.State.Set against that common
// schema shape, not through this interface.
type typedModel interface {
	Meta() typedMeta
	ToParams() map[string]string
	FromParams(params map[string]string)
}
```

Then `internal/provider/typed_tag_resource.go` (mirrors `resource_tagmanager_tag.go`'s CRUD bodies, but generic over `newModel`):

```go
// internal/provider/typed_tag_resource.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// typedTagCommon holds the fields every generated tag model declares
// identically (see tools/gen/templates/schema.go.tmpl). Reading/writing
// through this shape (via req.Plan.Get / resp.State.Set, which populate
// only the fields present in both the config's object type and this
// struct's tfsdk tags) lets the shared runtime handle common fields
// without per-type reflection, while each model's own ToParams/FromParams
// handles the type-specific remainder.
type typedTagCommon struct {
	ID              types.String   `tfsdk:"id"`
	ContainerID     types.String   `tfsdk:"container_id"`
	Name            types.String   `tfsdk:"name"`
	Status          types.String   `tfsdk:"status"`
	FireTriggerIDs  []types.String `tfsdk:"fire_trigger_ids"`
	BlockTriggerIDs []types.String `tfsdk:"block_trigger_ids"`
}

var (
	_ resource.Resource                = &typedTagResource{}
	_ resource.ResourceWithConfigure   = &typedTagResource{}
	_ resource.ResourceWithImportState = &typedTagResource{}
)

// typedTagResource is the single CRUD implementation shared by every
// generated matomo_tagmanager_tag_<type> resource. newModel constructs a
// fresh, zero-valued instance of that type's generated model.
type typedTagResource struct {
	client   *matomo.Client
	newModel func() typedModel
}

func newTypedTagResource(newModel func() typedModel) resource.Resource {
	return &typedTagResource{newModel: newModel}
}

func (r *typedTagResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	// Meta().ResourceName is already the full Terraform type name (e.g.
	// "matomo_tagmanager_tag_customhtml"), not a suffix to append to
	// req.ProviderTypeName - unlike the hand-written resources
	// (resource_tagmanager_tag.go etc.), which only ever have one type
	// name and so build it from the provider prefix at registration time.
	resp.TypeName = r.newModel().Meta().ResourceName
}

func (r *typedTagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.newModel().Meta().Schema
}

func (r *typedTagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	r.client = client
}

func (r *typedTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.Plan.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var common typedTagCommon
	resp.Diagnostics.Append(req.Plan.Get(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(common.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	fireIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(common.FireTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid fire_trigger_ids", err.Error())
		return
	}
	blockIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(common.BlockTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid block_trigger_ids", err.Error())
		return
	}

	idTag, err := r.client.AddContainerTag(ctx, siteID, idContainer, versionID, matomo.TagParams{
		Type:            model.Meta().TypeID,
		Name:            common.Name.ValueString(),
		Parameters:      model.ToParams(),
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager tag", err.Error())
		return
	}

	status := "active"
	if !common.Status.IsUnknown() && !common.Status.IsNull() {
		status = common.Status.ValueString()
	}
	if status == "paused" {
		if err := r.client.PauseContainerTag(ctx, siteID, idContainer, versionID, idTag); err != nil {
			resp.Diagnostics.AddError("Error pausing Matomo Tag Manager tag", err.Error())
			return
		}
	}

	common.ID = types.StringValue(buildEntityID(siteID, idContainer, idTag))
	common.Status = types.StringValue(status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var common typedTagCommon
	resp.Diagnostics.Append(req.State.Get(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	tag, err := r.client.GetContainerTag(ctx, siteID, idContainer, versionID, idTag)
	if err != nil {
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Tag does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager tag", err.Error())
		return
	}

	common.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	common.Name = types.StringValue(tag.Name)
	common.Status = types.StringValue(tag.Status)
	common.FireTriggerIDs = stringModelFromSlice(compositeEntityIDs(siteID, idContainer, intsToStrings(tag.FireTriggerIDs)))
	common.BlockTriggerIDs = stringModelFromSlice(compositeEntityIDs(siteID, idContainer, intsToStrings(tag.BlockTriggerIDs)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := r.newModel()
	model.FromParams(tag.Parameters)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.Plan.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var common typedTagCommon
	resp.Diagnostics.Append(req.Plan.Get(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	fireIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(common.FireTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid fire_trigger_ids", err.Error())
		return
	}
	blockIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(common.BlockTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid block_trigger_ids", err.Error())
		return
	}

	if err := r.client.UpdateContainerTag(ctx, siteID, idContainer, versionID, idTag, matomo.TagParams{
		Type:            model.Meta().TypeID,
		Name:            common.Name.ValueString(),
		Parameters:      model.ToParams(),
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager tag", err.Error())
		return
	}

	status := "active"
	if !common.Status.IsUnknown() && !common.Status.IsNull() {
		status = common.Status.ValueString()
	}
	if status == "paused" {
		err = r.client.PauseContainerTag(ctx, siteID, idContainer, versionID, idTag)
	} else {
		err = r.client.ResumeContainerTag(ctx, siteID, idContainer, versionID, idTag)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager tag status", err.Error())
		return
	}

	common.Status = types.StringValue(status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var common typedTagCommon
	resp.Diagnostics.Append(req.State.Get(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.DeleteContainerTag(ctx, siteID, idContainer, versionID, idTag); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager tag", err.Error())
	}
}

func (r *typedTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

`internal/provider/typed_trigger_resource.go` and `internal/provider/typed_variable_resource.go` follow the identical pattern, adapted to `matomo.TriggerParams`/`matomo.VariableParams` and their own common-field shapes (`typedTriggerCommon{ID, ContainerID, Name types.String}` with no status/fire-trigger fields; `typedVariableCommon{ID, ContainerID, Name, DefaultValue types.String}`), calling `AddContainerTrigger`/`GetContainerTrigger`/`UpdateContainerTrigger`/`DeleteContainerTrigger` and `AddContainerVariable`/`GetContainerVariable`/`UpdateContainerVariable`/`DeleteContainerVariable` respectively, and reusing the not-found detection each existing generic resource already has (`resource_tagmanager_trigger.go`, `resource_tagmanager_variable.go` — read both in full before writing these two files, and match their exact not-found error strings).

`internal/provider/typed_trigger_resource.go` and `internal/provider/typed_variable_resource.go`'s `Metadata` methods follow Task 8's already-fixed `typedTagResource.Metadata` pattern (assign `r.newModel().Meta().ResourceName` directly, no provider-prefix concatenation).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/provider/... -run TestTypedTagResource_metadataAndSchemaDispatchToModel -v`
Expected: PASS

Also run: `go build ./...` — expected to succeed except for `provider.go` (Task 10 hasn't wired `generatedResources()` yet, which doesn't exist until Task 9; this task's own new files must compile standalone).

- [ ] **Step 5: Commit**

```bash
git add internal/provider/typed_model.go internal/provider/typed_tag_resource.go internal/provider/typed_trigger_resource.go internal/provider/typed_variable_resource.go internal/provider/typed_tag_resource_test.go
git commit -m "Add shared typed-resource CRUD runtime for tag/trigger/variable"
```

---

## Task 9: `tools/gen` main — orchestration and file emission

**Files:**
- Create: `tools/gen/main.go`
- Create: `tools/gen/testscaffold.go`
- Create: `tools/gen/templates/acc_test.go.tmpl`

**Interfaces:**
- Consumes: `matomo.NewClient`/`GetAvailableTagTypes`/`GetAvailableTriggerTypes`/`GetAvailableVariableTypes` (Task 1), `BuildTypeSpec` (Task 5), `RenderSchema` (Task 7).
- Produces: the `tools/gen` CLI (`go run ./tools/gen`), which writes `internal/provider/generated/{tag,trigger,variable}_<slug>.go`, `internal/provider/generated/{tag,trigger,variable}_<slug>_acc_test.go` (only if absent), and `internal/provider/generated/generated_resources.go`.

- [ ] **Step 1: Write `tools/gen/main.go`**

```go
// tools/gen/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

const outputDir = "internal/provider/generated"

func main() {
	baseURL := os.Getenv("MATOMO_BASE_URL")
	apiToken := os.Getenv("MATOMO_API_TOKEN")
	if baseURL == "" || apiToken == "" {
		log.Fatal("tools/gen requires MATOMO_BASE_URL and MATOMO_API_TOKEN to be set (point them at the acceptance-test Matomo fixture)")
	}

	client := matomo.NewClient(baseURL, apiToken, &http.Client{})
	ctx := context.Background()

	specs, err := discoverAllSpecs(ctx, client)
	if err != nil {
		log.Fatalf("discovering Tag Manager types: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("creating %s: %v", outputDir, err)
	}

	for _, spec := range specs {
		if err := writeSchemaFile(spec); err != nil {
			log.Fatalf("writing schema file for %s %q: %v", spec.Kind, spec.TypeID, err)
		}
		if err := writeTestScaffoldIfAbsent(spec); err != nil {
			log.Fatalf("writing test scaffold for %s %q: %v", spec.Kind, spec.TypeID, err)
		}
	}

	if err := writeResourcesFile(specs); err != nil {
		log.Fatalf("writing generated_resources.go: %v", err)
	}

	log.Printf("generated %d typed Tag Manager resources into %s", len(specs), outputDir)
}

func discoverAllSpecs(ctx context.Context, client *matomo.Client) ([]TypeSpec, error) {
	var specs []TypeSpec

	tagTemplates, err := client.GetAvailableTagTypes(ctx, "web")
	if err != nil {
		return nil, fmt.Errorf("GetAvailableTagTypes: %w", err)
	}
	for _, tmpl := range tagTemplates {
		spec, err := BuildTypeSpec("tag", tmpl)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	triggerTemplates, err := client.GetAvailableTriggerTypes(ctx, "web")
	if err != nil {
		return nil, fmt.Errorf("GetAvailableTriggerTypes: %w", err)
	}
	for _, tmpl := range triggerTemplates {
		spec, err := BuildTypeSpec("trigger", tmpl)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	variableTemplates, err := client.GetAvailableVariableTypes(ctx, "web")
	if err != nil {
		return nil, fmt.Errorf("GetAvailableVariableTypes: %w", err)
	}
	for _, tmpl := range variableTemplates {
		spec, err := BuildTypeSpec("variable", tmpl)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

func writeSchemaFile(spec TypeSpec) error {
	src, err := RenderSchema(spec)
	if err != nil {
		return err
	}
	path := filepath.Join(outputDir, fmt.Sprintf("%s_%s.go", spec.Kind, spec.Slug))
	return os.WriteFile(path, src, 0o644)
}
```

- [ ] **Step 2: Write `tools/gen/templates/acc_test.go.tmpl` and `tools/gen/testscaffold.go`**

There is no shared container/trigger HCL helper in this codebase (confirmed by reading `resource_tagmanager_tag_acc_test.go` in full: every existing `TestAcc*` inlines a full `provider "matomo" {}` + `matomo_site` + `matomo_tagmanager_container` [+ `matomo_tagmanager_trigger`, for tag tests] config, and calls `testAccPreCheck(t)` first) — so the scaffold template does the same rather than inventing a helper that doesn't exist elsewhere:

```
{{/* tools/gen/templates/acc_test.go.tmpl */}}
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// This file is generated by tools/gen only if it does not already exist.
// Once present, tools/gen never overwrites it - edit freely to add real
// assertions (this scaffold only proves create+read-back with placeholder
// values).
func TestAcc{{.GoTypeName}}_createAndReadBack(t *testing.T) {
	testAccPreCheck(t)
	resourceName := "{{.ResourceName}}.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAcc{{.GoTypeName}}Config(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAcc{{.GoTypeName}}Config() string {
	return `
provider "matomo" {}

resource "matomo_site" "test" {
  name = "Generated {{.TypeID}} Acceptance Site"
  urls = ["https://acc-generated-{{.Slug}}.example.com"]
}

resource "matomo_tagmanager_container" "test" {
  site_id = matomo_site.test.id
  context = "web"
  name    = "Generated {{.TypeID}} Acceptance Container"
}
{{- if eq .Kind "tag"}}

resource "matomo_tagmanager_trigger" "test" {
  container_id = matomo_tagmanager_container.test.id
  type         = "PageView"
  name         = "Generated {{.TypeID}} Acceptance Trigger"
}
{{- end}}

resource "{{.ResourceName}}" "test" {
  container_id = matomo_tagmanager_container.test.id
  name         = "generated-test-{{.Slug}}"
{{- if eq .Kind "tag"}}
  fire_trigger_ids = [matomo_tagmanager_trigger.test.id]
{{- end}}
{{- range .Params}}
{{- if .Required}}
{{- if eq .GoType "String"}}
{{- if .AvailableValues}}
  {{.TFName}} = {{printf "%q" (index .AvailableValues 0)}}
{{- else}}
  {{.TFName}} = "test-value"
{{- end}}
{{- else if eq .GoType "Bool"}}
  {{.TFName}} = true
{{- end}}
{{- end}}
{{- end}}
}
`
}
```

```go
// tools/gen/testscaffold.go
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
)

var testScaffoldTemplate = template.Must(template.ParseFiles("tools/gen/templates/acc_test.go.tmpl"))

// writeTestScaffoldIfAbsent writes a minimal create+read-back acceptance
// test for spec, but only if that file doesn't already exist - so a
// hand-improved test is never clobbered by a later tools/gen run.
func writeTestScaffoldIfAbsent(spec TypeSpec) error {
	path := filepath.Join(outputDir, fmt.Sprintf("%s_%s_acc_test.go", spec.Kind, spec.Slug))
	if _, err := os.Stat(path); err == nil {
		return nil // already exists, leave it alone
	} else if !os.IsNotExist(err) {
		return err
	}

	var buf bytes.Buffer
	if err := testScaffoldTemplate.Execute(&buf, newTemplateData(spec)); err != nil {
		return fmt.Errorf("rendering test scaffold for %s %q: %w", spec.Kind, spec.TypeID, err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt-ing test scaffold for %s %q: %w\n--- unformatted source ---\n%s", spec.Kind, spec.TypeID, err, buf.String())
	}
	return os.WriteFile(path, formatted, 0o644)
}
```

- [ ] **Step 3: Write the `generated_resources.go` emitter**

```
{{/* tools/gen/templates/resources.go.tmpl */}}
// Code generated by tools/gen. DO NOT EDIT.

package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

// generatedResources returns one constructor per typed Tag Manager
// resource tools/gen discovered. Referenced from provider.go's
// Resources().
func generatedResources() []func() resource.Resource {
	return []func() resource.Resource{
{{- range .}}
{{- if eq .Kind "tag"}}
		func() resource.Resource { return newTypedTagResource(new{{.GoTypeName}}Model) },
{{- else if eq .Kind "trigger"}}
		func() resource.Resource { return newTypedTriggerResource(new{{.GoTypeName}}Model) },
{{- else if eq .Kind "variable"}}
		func() resource.Resource { return newTypedVariableResource(new{{.GoTypeName}}Model) },
{{- end}}
{{- end}}
	}
}
```

```go
// tools/gen/resources_file.go
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
)

var resourcesTemplate = template.Must(template.ParseFiles("tools/gen/templates/resources.go.tmpl"))

func writeResourcesFile(specs []TypeSpec) error {
	data := make([]templateData, len(specs))
	for i, spec := range specs {
		data[i] = newTemplateData(spec)
	}

	var buf bytes.Buffer
	if err := resourcesTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("rendering generated_resources.go: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt-ing generated_resources.go: %w\n--- unformatted source ---\n%s", err, buf.String())
	}
	return os.WriteFile(filepath.Join(outputDir, "generated_resources.go"), formatted, 0o644)
}
```

Add the corresponding call in `tools/gen/main.go`'s `main()` (already present in Step 1 above as `writeResourcesFile(specs)`).

- [ ] **Step 4: Verify it runs against the acceptance-test Matomo fixture**

Run:
```bash
docker compose up -d
./scripts/bootstrap-matomo.sh >> "$GITHUB_ENV" 2>/dev/null || ./scripts/bootstrap-matomo.sh
# export MATOMO_BASE_URL / MATOMO_API_TOKEN from the script's stdout, then:
go run ./tools/gen
```
Expected: exits 0, prints `generated N typed Tag Manager resources into internal/provider/generated`, and `internal/provider/generated/` contains real `.go` files. **This step will very likely fail at this point** with a `RequiredParams` error for every type beyond `CustomHtml`/`Constant` — that's expected and is exactly what Task 4's fail-loud design is for. Do not add a fallback in this task to suppress it; Task 12 is where every real discovered type gets a `requiredParams` entry. For this task's own verification, temporarily comment out `discoverAllSpecs`'s trigger/variable loops (or add a stub `requiredParams` entry of `nil` for each type discovered to unblock a smoke run only), confirm `internal/provider/generated/tag_customhtml.go` compiles (`go build ./internal/provider/generated/...`), then revert the temporary stub before committing.

- [ ] **Step 5: Commit**

```bash
git add tools/gen/main.go tools/gen/testscaffold.go tools/gen/resources_file.go tools/gen/templates/acc_test.go.tmpl tools/gen/templates/resources.go.tmpl
git commit -m "Add tools/gen orchestration: discover, build specs, emit files"
```

Do NOT commit anything under `internal/provider/generated/` in this task — that happens for real in Task 12, once `required.go` covers every discovered type.

---

## Task 10: Wire generated resources into the provider

**Files:**
- Modify: `internal/provider/provider.go:106-115`

**Interfaces:**
- Consumes: `generatedResources() []func() resource.Resource` (Task 9's `generated_resources.go`, which does not exist on disk until Task 12 actually runs `tools/gen` for real — this task's code compiles only once Task 12 has committed that generated file. Sequence this task's commit immediately after Task 12's in execution, not before.)

- [ ] **Step 1: Modify `Resources()`**

```go
// internal/provider/provider.go, replacing lines 106-115
func (p *MatomoProvider) Resources(_ context.Context) []func() resource.Resource {
	resources := []func() resource.Resource{
		NewSiteResource,
		NewCustomDimensionResource,
		NewTagManagerContainerResource,
		NewTagManagerTagResource,
		NewTagManagerTriggerResource,
		NewTagManagerVariableResource,
	}
	return append(resources, generatedResources()...)
}
```

- [ ] **Step 2: Confirm it compiles (only possible after Task 12 has generated `generated_resources.go`)**

Run: `go build ./...`
Expected: succeeds, and `terraform-plugin-framework`'s resource-type-name uniqueness check (exercised by `go test ./internal/provider/... -run TestProvider` if such a test exists in `provider_test.go`, or by running `terraform-plugin-docs`/`go vet` at minimum) reports no duplicate type names.

- [ ] **Step 3: Commit**

```bash
git add internal/provider/provider.go
git commit -m "Register generated typed Tag Manager resources with the provider"
```

---

## Task 11: CI — regenerate and diff

**Files:**
- Modify: `.github/workflows/acceptance.yml`

**Interfaces:**
- Consumes: the same Matomo fixture service and `MATOMO_BASE_URL`/`MATOMO_API_TOKEN` env vars the existing acceptance-test job already sets up (read the current job definition in full before editing, to match its exact service/health-check/env-var setup rather than inventing a new one).

- [ ] **Step 1: Add a regenerate-and-diff step**

Insert this step into the existing acceptance job in `.github/workflows/acceptance.yml`, immediately after the step that runs `scripts/bootstrap-matomo.sh` / sets `MATOMO_BASE_URL`/`MATOMO_API_TOKEN`, and before the step that runs `go test ./... -tags acceptance` (match whichever step name/order the file actually uses — read it first):

```yaml
      - name: Regenerate typed Tag Manager resources and check for drift
        run: |
          go run ./tools/gen
          if ! git diff --exit-code -- internal/provider/generated/; then
            echo "::error::internal/provider/generated/ is out of date. Run 'go run ./tools/gen' locally (against a Matomo instance) and commit the result." >&2
            exit 1
          fi
```

- [ ] **Step 2: Verify locally**

Run (with `docker compose up -d` and `scripts/bootstrap-matomo.sh`'s env vars exported, matching Task 9 Step 4):
```bash
go run ./tools/gen
git diff --exit-code -- internal/provider/generated/
echo $?
```
Expected: `0`, once Task 12 has committed fully up-to-date generated output — running this before Task 12 will correctly show a diff (or fail on `RequiredParams`) and is not expected to pass yet.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/acceptance.yml
git commit -m "Add CI drift check for tools/gen-generated resources"
```

---

## Task 12: Populate `required.go` for every real discovered type; generate and commit for real

This task is inherently data-driven: the exact set of types requiring a `requiredParams` entry can't be enumerated until `tools/gen` actually runs against a live Matomo instance (Task 9, Step 4 already surfaced the first failure). This task is the one-time research pass the design spec calls for, done for real rather than deferred further.

**Files:**
- Modify: `tools/gen/required.go`
- Create: `internal/provider/generated/*.go` (committed for real this time)
- Create/modify: `internal/provider/generated/*_acc_test.go` (scaffolds; hand-improve the ones needing more than placeholder values)

- [ ] **Step 1: Run `tools/gen` and capture the first failure**

```bash
docker compose up -d
# export MATOMO_BASE_URL / MATOMO_API_TOKEN per scripts/bootstrap-matomo.sh
go run ./tools/gen
```
Expected: fails with `no required-params entry for tag type "X" - read its Matomo source...` (or trigger/variable) for the first type beyond `CustomHtml`/`Constant`.

- [ ] **Step 2: For the failing type, read its Matomo source and add a `required.go` entry**

Find the type's source file in `matomo-org/tag-manager` (`5.x-dev` branch) under `Template/Tag/`, `Template/Trigger/`, or `Template/Variable/` (filename usually matches the type id, e.g. `Template/Tag/Ga4.php` for a `Ga4` tag type — confirm the exact filename via GitHub's file listing rather than guessing). Read its `configureTagFields`/`configureTriggerFields`/`configureVariableFields`-equivalent method (the method that calls `$field = $this->makeSetting(...)` per parameter) and note which fields have `$field->validators[] = new NotEmpty();` (or another required-implying validator) attached. Add one entry to `tools/gen/required.go`'s `requiredParams` map for that type, e.g.:

```go
	"tag": {
		"CustomHtml": {"customHtml"},
		"Ga4":        {"measurementId"},
	},
```

If the type has no required fields at all, still add an explicit empty-slice entry (`"SomeType": {},`) — required by `RequiredParams`'s fail-loud contract from Task 4.

- [ ] **Step 3: Repeat Steps 1-2 until `tools/gen` completes successfully**

Re-run `go run ./tools/gen` after each `required.go` addition; repeat until it finishes without error and prints `generated N typed Tag Manager resources`.

- [ ] **Step 4: Verify generated code compiles**

Run: `go build ./...`
Expected: succeeds (Task 10's `generated_resources.go` reference now resolves).

- [ ] **Step 5: Run every generated acceptance test**

Run: `TF_ACC=1 go test ./internal/provider/... -run TestAcc -v -timeout 30m`
Expected: PASS for every generated `TestAcc<Type>_createAndReadBack` test. Any failure here is either (a) a genuine wire-format bug (diagnose it the same way this project's earlier acceptance-testing work did: read the exact Matomo error, check real Matomo PHP source, fix the specific generator/runtime bug — do not guess), or (b) a required-field annotation that's actually wrong (Matomo may reject a create because the placeholder value or omitted-but-actually-required field is invalid) — re-check that type's source and fix `required.go`, then re-run from Step 3.

- [ ] **Step 6: Commit**

```bash
git add tools/gen/required.go internal/provider/generated/
git commit -m "Generate and commit typed Tag Manager resources for all discovered types"
```

- [ ] **Step 7: Run the full acceptance suite once more, end to end**

Run: `TF_ACC=1 go test ./... -v -timeout 30m`
Expected: PASS across the whole suite (existing generic-resource tests + all newly generated typed-resource tests), confirming Task 10's provider wiring and Task 11's CI diff check are both consistent with what's committed.

- [ ] **Step 8: Push and confirm CI is green**

```bash
git push -u origin <branch-name>
```
Then watch the `acceptance.yml` run (per this project's established pattern of watching real CI rather than assuming success) to confirm the new "Regenerate typed Tag Manager resources and check for drift" step passes with zero diff, and the full acceptance suite passes.
