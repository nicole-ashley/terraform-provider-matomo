// tools/gen/condition_render.go
package main

import (
	"fmt"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// renderCondition renders a parsed matomo.ConditionNode (its field names
// already rewritten to TF snake_case by rewriteConditionFieldsToTFNames)
// into a Go source expression constructing the equivalent
// matomo.ConditionNode value, for embedding directly into a generated
// file's conditionRequiredValidator{Condition: ...} literal - the
// generated file can't call ParseCondition itself (that only exists in
// tools/gen), so the already-parsed tree is baked in as Go source instead.
func renderCondition(node matomo.ConditionNode) string {
	switch n := node.(type) {
	case matomo.RefNode:
		return fmt.Sprintf("matomo.RefNode{Field: %q}", n.Field)
	case matomo.EqNode:
		return fmt.Sprintf("matomo.EqNode{Field: %q, Value: %q, Negate: %v}", n.Field, n.Value, n.Negate)
	case matomo.NotNode:
		return fmt.Sprintf("matomo.NotNode{Inner: %s}", renderCondition(n.Inner))
	case matomo.AndNode:
		return fmt.Sprintf("matomo.AndNode{Left: %s, Right: %s}", renderCondition(n.Left), renderCondition(n.Right))
	case matomo.OrNode:
		return fmt.Sprintf("matomo.OrNode{Left: %s, Right: %s}", renderCondition(n.Left), renderCondition(n.Right))
	default:
		panic(fmt.Sprintf("renderCondition: unhandled condition node type %T", node))
	}
}
