package tsl

import "github.com/yaacov/tree-search-language/v6/pkg/parser"

// convertNodeKind converts parser NodeKind to TSL Kind.
// Both types use matching iota values so this is a direct cast.
func convertNodeKind(parserKind parser.NodeKind) Kind {
	return Kind(parserKind)
}

// convertOpType converts parser OpType to TSL Operator.
// Both types use matching iota+100 values so this is a direct cast.
func convertOpType(parserOp parser.OpType) Operator {
	return Operator(parserOp)
}

// wrapParserNode creates a TSL node from a parser node
func wrapParserNode(parserNode *parser.Node) *Node {
	if parserNode == nil {
		return nil
	}

	tslNode := &Node{
		Kind:     convertNodeKind(parserNode.Kind),
		Value:    parserNode.Value,
		Operator: convertOpType(parserNode.Operator),
		Position: parserNode.Position,
		Left:     wrapParserNode(parserNode.Left),
		Right:    wrapParserNode(parserNode.Right),
	}

	// Convert children array if present
	if parserNode.Children != nil {
		tslNode.Children = make([]*Node, len(parserNode.Children))
		for i, child := range parserNode.Children {
			tslNode.Children[i] = wrapParserNode(child)
		}
	}

	return tslNode
}

// Node represents a TSL AST node with semantic type information
type Node struct {
	Kind Kind
	// Value must be an immutable type (string, float64, bool, time.Time, or nil).
	// Clone performs a shallow copy of this field.
	Value    interface{}
	Operator Operator
	Left     *Node
	Right    *Node
	Children []*Node
	Position int
}

// Clone creates a deep copy of the TSL node
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}

	clone := &Node{
		Kind:     n.Kind,
		Value:    n.Value,
		Operator: n.Operator,
		Position: n.Position,
		Left:     n.Left.Clone(),
		Right:    n.Right.Clone(),
	}

	if n.Children != nil {
		clone.Children = make([]*Node, len(n.Children))
		for i, child := range n.Children {
			clone.Children[i] = child.Clone()
		}
	}

	return clone
}
