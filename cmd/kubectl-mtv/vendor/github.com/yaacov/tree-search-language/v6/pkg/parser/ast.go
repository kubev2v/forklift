package parser

import (
	"fmt"
	"strconv"
	"time"
)

// Generic parser node types (no TSL-specific semantics)

// NodeKind represents the kind of node in a generic AST
type NodeKind int

const (
	NodeNumericLiteral NodeKind = iota
	NodeStringLiteral
	NodeIdentifier
	NodeBinaryExpr
	NodeUnaryExpr
	NodeDateLiteral
	NodeTimestampLiteral
	NodeArrayLiteral
	NodeBooleanLiteral
	NodeNullLiteral
)

// String returns the string representation of NodeKind
func (nk NodeKind) String() string {
	switch nk {
	case NodeNumericLiteral:
		return "NUMBER"
	case NodeStringLiteral:
		return "STRING"
	case NodeIdentifier:
		return "IDENTIFIER"
	case NodeBinaryExpr:
		return "BINARY_EXPR"
	case NodeUnaryExpr:
		return "UNARY_EXPR"
	case NodeDateLiteral:
		return "DATE"
	case NodeTimestampLiteral:
		return "TIMESTAMP"
	case NodeArrayLiteral:
		return "ARRAY"
	case NodeBooleanLiteral:
		return "BOOLEAN"
	case NodeNullLiteral:
		return "NULL"
	default:
		return "UNKNOWN"
	}
}

// OpType represents a generic operator type
type OpType int

const (
	OpEQ OpType = iota + 100 // Start from 100 to avoid conflicts
	OpNE
	OpLT
	OpLE
	OpGT
	OpGE
	OpLike
	OpILike
	OpAnd
	OpOr
	OpNot
	OpIn
	OpBetween
	OpIs
	OpPlus
	OpMinus
	OpStar
	OpSlash
	OpPercent
	OpLen
	OpAny
	OpAll
	OpSum
	OpREQ
	OpRNE
	OpUMinus
)

// String returns the string representation of OpType
func (op OpType) String() string {
	switch op {
	case OpEQ:
		return "="
	case OpNE:
		return "!="
	case OpLT:
		return "<"
	case OpLE:
		return "<="
	case OpGT:
		return ">"
	case OpGE:
		return ">="
	case OpLike:
		return "LIKE"
	case OpILike:
		return "ILIKE"
	case OpAnd:
		return "AND"
	case OpOr:
		return "OR"
	case OpNot:
		return "NOT"
	case OpIn:
		return "IN"
	case OpBetween:
		return "BETWEEN"
	case OpIs:
		return "IS"
	case OpPlus:
		return "+"
	case OpMinus:
		return "-"
	case OpStar:
		return "*"
	case OpSlash:
		return "/"
	case OpPercent:
		return "%"
	case OpLen:
		return "LEN"
	case OpAny:
		return "ANY"
	case OpAll:
		return "ALL"
	case OpSum:
		return "SUM"
	case OpREQ:
		return "~="
	case OpRNE:
		return "~!"
	case OpUMinus:
		return "NEG"
	default:
		return "UNKNOWN"
	}
}

// Node represents a generic AST node
type Node struct {
	Kind     NodeKind
	Value    interface{}
	Operator OpType
	Left     *Node
	Right    *Node
	Children []*Node
	Position int // Position in the input string for error reporting
}

// parseSizeValue converts size strings like "5k", "2M", "1G" to numeric values
func parseSizeValue(value string) (float64, error) {
	if len(value) == 0 {
		return 0, fmt.Errorf("empty value")
	}

	// Check for size suffixes
	lastChar := value[len(value)-1]
	var multiplier float64
	var numStr string

	switch lastChar {
	case 'k', 'K':
		multiplier = 1000
		numStr = value[:len(value)-1]
	case 'm', 'M':
		multiplier = 1000 * 1000
		numStr = value[:len(value)-1]
	case 'g', 'G':
		multiplier = 1000 * 1000 * 1000
		numStr = value[:len(value)-1]
	case 't', 'T':
		multiplier = 1000 * 1000 * 1000 * 1000
		numStr = value[:len(value)-1]
	case 'p', 'P':
		multiplier = 1000 * 1000 * 1000 * 1000 * 1000
		numStr = value[:len(value)-1]
	case 'i', 'I':
		// Handle binary prefixes like "Ki", "Mi", "Gi", "Ti", "Pi"
		if len(value) < 2 {
			return strconv.ParseFloat(value, 64)
		}
		secondLastChar := value[len(value)-2]
		switch secondLastChar {
		case 'k', 'K':
			multiplier = 1024
			numStr = value[:len(value)-2]
		case 'm', 'M':
			multiplier = 1024 * 1024
			numStr = value[:len(value)-2]
		case 'g', 'G':
			multiplier = 1024 * 1024 * 1024
			numStr = value[:len(value)-2]
		case 't', 'T':
			multiplier = 1024 * 1024 * 1024 * 1024
			numStr = value[:len(value)-2]
		case 'p', 'P':
			multiplier = 1024 * 1024 * 1024 * 1024 * 1024
			numStr = value[:len(value)-2]
		default:
			// Not a valid binary prefix, parse as regular number
			return strconv.ParseFloat(value, 64)
		}
	default:
		// No suffix, parse as regular number
		return strconv.ParseFloat(value, 64)
	}

	// Parse the numeric part
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	return num * multiplier, nil
}

// NewNumberNode creates a numeric literal node
func NewNumberNode(value string, pos int) *Node {
	// Parse as float64 to handle all numeric types consistently, including size suffixes
	val, err := parseSizeValue(value)
	if err != nil {
		val = 0.0
	}
	return &Node{
		Kind:     NodeNumericLiteral,
		Value:    val,
		Position: pos,
	}
}

// NewStringNode creates a string literal node
func NewStringNode(value string, pos int) *Node {
	return &Node{
		Kind:     NodeStringLiteral,
		Value:    value,
		Position: pos,
	}
}

// NewIdentifierNode creates an identifier node
func NewIdentifierNode(value string, pos int) *Node {
	return &Node{
		Kind:     NodeIdentifier,
		Value:    value,
		Position: pos,
	}
}

// NewBooleanNode creates a boolean literal node
func NewBooleanNode(value bool, pos int) *Node {
	return &Node{
		Kind:     NodeBooleanLiteral,
		Value:    value,
		Position: pos,
	}
}

// NewNullNode creates a null literal node
func NewNullNode(pos int) *Node {
	return &Node{
		Kind:     NodeNullLiteral,
		Value:    nil,
		Position: pos,
	}
}

// NewDateNode creates a date literal node
func NewDateNode(value string, pos int) *Node {
	// Store as string for proper display formatting
	// Parsing validation will be done by consumers when needed
	return &Node{
		Kind:     NodeDateLiteral,
		Value:    value,
		Position: pos,
	}
}

// NewTimestampNode creates a timestamp literal node (RFC3339)
func NewTimestampNode(value string, pos int) *Node {
	// Try to parse as time.Time
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &Node{
			Kind:     NodeTimestampLiteral,
			Value:    t,
			Position: pos,
		}
	}
	// Fallback to string
	return &Node{
		Kind:     NodeTimestampLiteral,
		Value:    value,
		Position: pos,
	}
}

// NewBinaryOpNode creates a binary operation node
func NewBinaryOpNode(op OpType, left, right *Node, pos int) *Node {
	return &Node{
		Kind:     NodeBinaryExpr,
		Operator: op,
		Left:     left,
		Right:    right,
		Position: pos,
	}
}

// NewUnaryOpNode creates a unary operation node
func NewUnaryOpNode(op OpType, child *Node, pos int) *Node {
	return &Node{
		Kind:     NodeUnaryExpr,
		Operator: op,
		Right:    child, // Store unary operand in Right for consistency
		Position: pos,
	}
}

// NewArrayNode creates an array literal node
func NewArrayNode(elements []*Node, pos int) *Node {
	return &Node{
		Kind:     NodeArrayLiteral,
		Children: elements,
		Position: pos,
	}
}

// Clone creates a deep copy of the node and its children
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}

	clone := &Node{
		Kind:     n.Kind,
		Value:    n.Value,
		Operator: n.Operator,
		Position: n.Position,
	}

	if n.Left != nil {
		clone.Left = n.Left.Clone()
	}
	if n.Right != nil {
		clone.Right = n.Right.Clone()
	}
	if n.Children != nil {
		clone.Children = make([]*Node, len(n.Children))
		for i, child := range n.Children {
			clone.Children[i] = child.Clone()
		}
	}

	return clone
}

// String returns a string representation of the node (for debugging)
func (n *Node) String() string {
	if n == nil {
		return "<nil>"
	}

	switch n.Kind {
	case NodeNumericLiteral, NodeStringLiteral, NodeIdentifier,
		NodeDateLiteral, NodeTimestampLiteral, NodeBooleanLiteral:
		return fmt.Sprintf("%s(%v)", n.Kind, n.Value)
	case NodeNullLiteral:
		return "NULL"
	case NodeBinaryExpr:
		return fmt.Sprintf("(%s %s %s)", n.Left, n.Operator, n.Right)
	case NodeUnaryExpr:
		return fmt.Sprintf("(%s %s)", n.Operator, n.Right)
	case NodeArrayLiteral:
		result := "["
		for i, child := range n.Children {
			if i > 0 {
				result += ", "
			}
			result += child.String()
		}
		result += "]"
		return result
	default:
		return fmt.Sprintf("UNKNOWN(%v)", n.Value)
	}
}
