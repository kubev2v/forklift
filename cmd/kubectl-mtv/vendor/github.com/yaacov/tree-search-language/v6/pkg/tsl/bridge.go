package tsl

import "github.com/yaacov/tree-search-language/v6/pkg/parser"

// Type mapping tables for TSL semantic types
var (
	nodeKindMap = map[parser.NodeKind]Kind{
		parser.NodeNumericLiteral:   KindNumericLiteral,
		parser.NodeStringLiteral:    KindStringLiteral,
		parser.NodeIdentifier:       KindIdentifier,
		parser.NodeBinaryExpr:       KindBinaryExpr,
		parser.NodeUnaryExpr:        KindUnaryExpr,
		parser.NodeDateLiteral:      KindDateLiteral,
		parser.NodeTimestampLiteral: KindTimestampLiteral,
		parser.NodeArrayLiteral:     KindArrayLiteral,
		parser.NodeBooleanLiteral:   KindBooleanLiteral,
		parser.NodeNullLiteral:      KindNullLiteral,
	}

	operatorMap = map[parser.OpType]Operator{
		parser.OpEQ:      OpEQ,
		parser.OpNE:      OpNE,
		parser.OpLT:      OpLT,
		parser.OpLE:      OpLE,
		parser.OpGT:      OpGT,
		parser.OpGE:      OpGE,
		parser.OpLike:    OpLike,
		parser.OpILike:   OpILike,
		parser.OpAnd:     OpAnd,
		parser.OpOr:      OpOr,
		parser.OpNot:     OpNot,
		parser.OpIn:      OpIn,
		parser.OpBetween: OpBetween,
		parser.OpIs:      OpIs,
		parser.OpPlus:    OpPlus,
		parser.OpMinus:   OpMinus,
		parser.OpStar:    OpStar,
		parser.OpSlash:   OpSlash,
		parser.OpPercent: OpPercent,
		parser.OpLen:     OpLen,
		parser.OpAny:     OpAny,
		parser.OpAll:     OpAll,
		parser.OpSum:     OpSum,
		parser.OpREQ:     OpREQ,
		parser.OpRNE:     OpRNE,
		parser.OpUMinus:  OpUMinus,
	}
)

// convertNodeKind converts parser NodeKind to TSL Kind
func convertNodeKind(parserKind parser.NodeKind) Kind {
	if kind, ok := nodeKindMap[parserKind]; ok {
		return kind
	}
	return KindStringLiteral // fallback
}

// convertOpType converts parser OpType to TSL Operator
func convertOpType(parserOp parser.OpType) Operator {
	if op, ok := operatorMap[parserOp]; ok {
		return op
	}
	return OpEQ // fallback
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
	Kind     Kind
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
