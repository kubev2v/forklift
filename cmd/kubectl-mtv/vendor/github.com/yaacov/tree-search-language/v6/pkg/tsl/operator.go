package tsl

// Operator represents the type definitions.
// Values match parser.OpType so conversion is a simple cast.
type Operator int

const (
	// Comparison operators
	OpEQ Operator = iota + 100
	OpNE
	OpLT
	OpLE
	OpGT
	OpGE

	// String operators
	OpLike
	OpILike

	// Logical operators
	OpAnd
	OpOr
	OpNot

	// Array/set operators
	OpIn
	OpBetween

	// Null operator
	OpIs

	// Arithmetic operators
	OpPlus
	OpMinus
	OpStar
	OpSlash
	OpPercent

	// Unary operators
	OpLen
	OpAny
	OpAll
	OpSum

	// Regex operators
	OpREQ
	OpRNE

	// Unary minus
	OpUMinus
)

// String returns the string representation of an OperatorType
func (op Operator) String() string {
	switch op {
	// Comparison Operators
	case OpEQ:
		return "EQ"
	case OpNE:
		return "NE"
	case OpLT:
		return "LT"
	case OpLE:
		return "LE"
	case OpGT:
		return "GT"
	case OpGE:
		return "GE"
	case OpREQ:
		return "REQ"
	case OpRNE:
		return "RNE"

	// Logical Operators
	case OpAnd:
		return "AND"
	case OpOr:
		return "OR"
	case OpNot:
		return "NOT"

	// String Operators
	case OpLike:
		return "LIKE"
	case OpILike:
		return "ILIKE"

	// Array Operators
	case OpIn:
		return "IN"
	case OpBetween:
		return "BETWEEN"

	// Null Operators
	case OpIs:
		return "IS"

	// Arithmetic Operators
	case OpPlus:
		return "ADD"
	case OpMinus:
		return "SUB"
	case OpStar:
		return "MUL"
	case OpSlash:
		return "DIV"
	case OpPercent:
		return "MOD"

	// Unary Operators
	case OpUMinus:
		return "NEG"
	case OpLen:
		return "LEN"
	case OpAny:
		return "ANY"
	case OpAll:
		return "ALL"
	case OpSum:
		return "SUM"

	default:
		return "UNKNOWN"
	}
}
