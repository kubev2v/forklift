package parser

// tslLexer adapts our Lexer to the goyacc interface
type tslLexer struct {
	lexer *Lexer
	pos   int
}

// Lex implements the goyacc lexer interface
func (l *tslLexer) Lex(lval *yySymType) int {
	token := l.lexer.NextToken()
	l.pos = token.Position

	// Set the semantic value for string tokens
	switch token.Type {
	default:
		if token.Value != "" {
			lval.str = token.Value
		}
	}

	return token.Type
}

// Error implements the goyacc lexer interface
func (l *tslLexer) Error(s string) {
	parseError = &ParseError{
		Message:  s,
		Position: l.pos,
	}
}

// Parse parses a TSL expression and returns the AST
func Parse(input string) (*Node, error) {
	// Reset global state
	parseResult = nil
	parseError = nil

	// Create and tokenize
	currentLexer = NewLexer(input)
	if err := currentLexer.Tokenize(); err != nil {
		return nil, err
	}

	// Create goyacc lexer adapter
	yylex := &tslLexer{lexer: currentLexer}

	// Parse
	if yyParse(yylex) != 0 {
		if parseError != nil {
			return nil, parseError
		}
		return nil, &ParseError{
			Message:  "Parse error",
			Position: 0,
		}
	}

	return parseResult, nil
}

// Initialize keyword map with correct token constants
func init() {
	keywords["like"] = K_LIKE
	keywords["ilike"] = K_ILIKE
	keywords["and"] = K_AND
	keywords["or"] = K_OR
	keywords["between"] = K_BETWEEN
	keywords["in"] = K_IN
	keywords["is"] = K_IS
	keywords["null"] = K_NULL
	keywords["not"] = K_NOT
	keywords["true"] = K_TRUE
	keywords["false"] = K_FALSE
	keywords["len"] = K_LEN
	keywords["any"] = K_ANY
	keywords["all"] = K_ALL
	keywords["sum"] = K_SUM
}
