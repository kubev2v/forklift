package tsl

// Kind represents the kind of node in the AST.
// Values match parser.NodeKind so conversion is a simple cast.
type Kind int

const (
	KindInvalid Kind = -1
)

const (
	KindNumericLiteral Kind = iota
	KindStringLiteral
	KindIdentifier
	KindBinaryExpr
	KindUnaryExpr
	KindDateLiteral
	KindTimestampLiteral
	KindArrayLiteral
	KindBooleanLiteral
	KindNullLiteral
)

// String returns the string representation of a NodeKind
func (kind Kind) String() string {
	switch kind {
	case KindStringLiteral:
		return "STRING"
	case KindNumericLiteral:
		return "NUMBER"
	case KindBooleanLiteral:
		return "BOOLEAN"
	case KindNullLiteral:
		return "NULL"
	case KindDateLiteral:
		return "DATE"
	case KindTimestampLiteral:
		return "TIMESTAMP"
	case KindArrayLiteral:
		return "ARRAY"
	case KindIdentifier:
		return "IDENTIFIER"
	case KindBinaryExpr:
		return "BINARY_EXP"
	case KindUnaryExpr:
		return "UNARY_EXP"
	case KindInvalid:
		return "INVALID"
	default:
		return "UNKNOWN"
	}
}
