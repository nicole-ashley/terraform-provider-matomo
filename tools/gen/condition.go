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
