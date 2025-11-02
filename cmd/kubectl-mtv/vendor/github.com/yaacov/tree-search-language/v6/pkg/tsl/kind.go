package tsl

// Kind represents the kind of node in the AST
// These values must match the ast_node_type enum in tsl_ast.h
type Kind int

const (
	KindNumericLiteral   Kind = 0 // AST_NUMBER
	KindStringLiteral    Kind = 1 // AST_STRING
	KindIdentifier       Kind = 2 // AST_IDENTIFIER
	KindBinaryExpr       Kind = 3 // AST_BINARY_OP
	KindUnaryExpr        Kind = 4 // AST_UNARY_OP
	KindDateLiteral      Kind = 5 // AST_DATE
	KindTimestampLiteral Kind = 6 // AST_RFC3339
	KindArrayLiteral     Kind = 7 // AST_ARRAY
	KindBooleanLiteral   Kind = 8 // AST_BOOL
	KindNullLiteral      Kind = 9 // AST_NULL
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
	default:
		return "UNKNOWN"
	}
}
