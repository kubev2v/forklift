package parser

import (
	"regexp"
	"strings"
	"unicode"
)

// Token represents a lexical token
type Token struct {
	Type     int    // Token type (matches yacc token constants)
	Value    string // Token value/text
	Position int    // Position in input string
}

// Token constants - these will match the generated constants from goyacc
const (
	EOF     = 0
	ILLEGAL = -1
)

// Lexer represents a lexical analyzer for TSL
type Lexer struct {
	input   string
	pos     int // current position
	start   int // start of current token
	tokens  []Token
	current int // current token index
}

// Keywords map (case-insensitive) - values will be set after parser generation
var keywords = map[string]int{
	"like":    1, // Will be updated to match generated constants
	"ilike":   1,
	"and":     1,
	"or":      1,
	"between": 1,
	"in":      1,
	"is":      1,
	"null":    1,
	"not":     1,
	"true":    1,
	"false":   1,
	"len":     1,
	"any":     1,
	"all":     1,
	"sum":     1,
}

// Regular expressions for token patterns
var (
	// Date and time patterns
	datePattern    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	rfc3339Pattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)
)

// NewLexer creates a new lexer instance
func NewLexer(input string) *Lexer {
	return &Lexer{
		input:   input,
		pos:     0,
		start:   0,
		tokens:  make([]Token, 0),
		current: 0,
	}
}

// Tokenize scans the input and produces tokens
func (l *Lexer) Tokenize() error {
	for !l.isAtEnd() {
		l.start = l.pos
		if err := l.scanToken(); err != nil {
			return err
		}
	}

	l.addToken(EOF, "")
	return nil
}

// isAtEnd checks if we're at the end of input
func (l *Lexer) isAtEnd() bool {
	return l.pos >= len(l.input)
}

// peek returns the current character without advancing
func (l *Lexer) peek() rune {
	if l.isAtEnd() {
		return 0
	}
	return rune(l.input[l.pos])
}

// peekNext returns the next character without advancing
func (l *Lexer) peekNext() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return rune(l.input[l.pos+1])
}

// advance consumes and returns the current character
func (l *Lexer) advance() rune {
	if l.isAtEnd() {
		return 0
	}
	c := rune(l.input[l.pos])
	l.pos++
	return c
}

// match checks if current character matches expected and advances if so
func (l *Lexer) match(expected rune) bool {
	if l.isAtEnd() || rune(l.input[l.pos]) != expected {
		return false
	}
	l.pos++
	return true
}

// addToken adds a token to the token list
func (l *Lexer) addToken(tokenType int, value string) {
	l.tokens = append(l.tokens, Token{
		Type:     tokenType,
		Value:    value,
		Position: l.start,
	})
}

// scanToken scans a single token
func (l *Lexer) scanToken() error {
	c := l.advance()

	switch c {
	case ' ', '\r', '\t', '\n', '\f', '\v':
		// Skip whitespace
		return nil
	case '(':
		l.addToken(LPAREN, "(")
	case ')':
		l.addToken(RPAREN, ")")
	case '[':
		l.addToken(LBRACKET, "[")
	case ']':
		l.addToken(RBRACKET, "]")
	case ',':
		l.addToken(COMMA, ",")
	case '+':
		l.addToken(PLUS, "+")
	case '-':
		l.addToken(MINUS, "-")
	case '*':
		l.addToken(STAR, "*")
	case '/':
		l.addToken(SLASH, "/")
	case '%':
		l.addToken(PERCENT, "%")
	case '=':
		l.addToken(EQ, "=")
	case '<':
		if l.match('=') {
			l.addToken(LE, "<=")
		} else if l.match('>') {
			l.addToken(NE, "<>")
		} else {
			l.addToken(LT, "<")
		}
	case '>':
		if l.match('=') {
			l.addToken(GE, ">=")
		} else {
			l.addToken(GT, ">")
		}
	case '!':
		if l.match('=') {
			l.addToken(NE, "!=")
		} else {
			return &ParseError{
				Message:  "Unexpected character '!'",
				Position: l.start,
			}
		}
	case '~':
		if l.match('=') {
			l.addToken(REQ, "~=")
		} else if l.match('!') {
			l.addToken(RNE, "~!")
		} else {
			return &ParseError{
				Message:  "Unexpected character '~'",
				Position: l.start,
			}
		}
	case '\'':
		return l.scanString('\'')
	case '"':
		return l.scanString('"')
	case '`':
		return l.scanString('`')
	default:
		if unicode.IsDigit(c) || (c == '-' && unicode.IsDigit(l.peek())) || (c == '+' && unicode.IsDigit(l.peek())) {
			// Put back the character and check if it's a date/time pattern first
			l.pos--
			if l.isDateTimePattern() {
				return l.scanDateTime()
			}
			return l.scanNumber()
		} else if unicode.IsLetter(c) || c == '_' {
			// Put back the character and scan identifier/keyword
			l.pos--
			return l.scanIdentifier()
		} else {
			return &ParseError{
				Message:  "Unexpected character '" + string(c) + "'",
				Position: l.start,
			}
		}
	}

	return nil
}

// isDateTimePattern checks if the current position starts a date or time pattern
func (l *Lexer) isDateTimePattern() bool {
	// Look ahead to see if this matches a date or RFC3339 pattern
	remaining := l.input[l.pos:]

	// Try to match date pattern (YYYY-MM-DD)
	if len(remaining) >= 10 {
		dateCandidate := remaining[:10]
		if datePattern.MatchString(dateCandidate) {
			// Check if it continues as RFC3339 (has T after date)
			if len(remaining) > 10 && remaining[10] == 'T' {
				// Look for a complete RFC3339 pattern
				for i := 10; i < len(remaining); i++ {
					c := remaining[i]
					if c == ' ' || c == '\t' || c == '\n' || c == ')' || c == ',' {
						// End of potential timestamp
						candidate := remaining[:i]
						return rfc3339Pattern.MatchString(candidate)
					}
				}
				// Check the whole remaining string
				return rfc3339Pattern.MatchString(remaining)
			}
			return true // Just a date
		}
	}

	return false
}

// scanDateTime scans a date or RFC3339 timestamp
func (l *Lexer) scanDateTime() error {
	start := l.pos

	// Scan until we find a delimiter or end
	for !l.isAtEnd() {
		c := l.peek()
		if c == ' ' || c == '\t' || c == '\n' || c == ')' || c == ',' || c == ']' {
			break
		}
		l.advance()
	}

	value := l.input[start:l.pos]

	// Check if it's a date or RFC3339 format
	if rfc3339Pattern.MatchString(value) {
		l.addToken(RFC3339, value)
	} else if datePattern.MatchString(value) {
		l.addToken(DATE, value)
	} else {
		// Fallback: might be a number with some unexpected format
		// Put the position back and let scanNumber handle it
		l.pos = start
		return l.scanNumber()
	}

	return nil
}

// scanString scans a quoted string literal
func (l *Lexer) scanString(quote rune) error {
	var value strings.Builder

	for !l.isAtEnd() && l.peek() != quote {
		c := l.advance()
		if c == '\\' && !l.isAtEnd() {
			// Handle escape sequences
			escaped := l.advance()
			switch escaped {
			case 'n':
				value.WriteRune('\n')
			case 't':
				value.WriteRune('\t')
			case 'r':
				value.WriteRune('\r')
			case 'b':
				value.WriteRune('\b')
			case 'f':
				value.WriteRune('\f')
			case '\\':
				value.WriteRune('\\')
			case '\'':
				value.WriteRune('\'')
			case '"':
				value.WriteRune('"')
			case '`':
				value.WriteRune('`')
			default:
				value.WriteRune(escaped)
			}
		} else {
			value.WriteRune(c)
		}
	}

	if l.isAtEnd() {
		return &ParseError{
			Message:  "Unterminated string",
			Position: l.start,
		}
	}

	// Consume closing quote
	l.advance()

	str := value.String()

	// Check if it's a date/time format
	if datePattern.MatchString(str) {
		l.addToken(DATE, str)
	} else if rfc3339Pattern.MatchString(str) {
		l.addToken(RFC3339, str)
	} else {
		l.addToken(STRING_LITERAL, str)
	}

	return nil
}

// scanNumber scans a numeric literal
func (l *Lexer) scanNumber() error {
	start := l.pos

	// Handle optional sign
	if l.peek() == '+' || l.peek() == '-' {
		l.advance()
	}

	// Scan digits
	for !l.isAtEnd() && unicode.IsDigit(l.peek()) {
		l.advance()
	}

	// Handle decimal point
	if !l.isAtEnd() && l.peek() == '.' && unicode.IsDigit(l.peekNext()) {
		l.advance() // consume '.'
		for !l.isAtEnd() && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}

	// Handle scientific notation
	if !l.isAtEnd() && (l.peek() == 'E' || l.peek() == 'e') {
		l.advance() // consume 'E' or 'e'
		if !l.isAtEnd() && (l.peek() == '+' || l.peek() == '-') {
			l.advance() // consume optional sign
		}
		for !l.isAtEnd() && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}

	// Handle size suffixes (k, M, G, T, P with optional 'i')
	if !l.isAtEnd() {
		c := l.peek()
		if c == 'k' || c == 'K' || c == 'm' || c == 'M' ||
			c == 'g' || c == 'G' || c == 't' || c == 'T' ||
			c == 'p' || c == 'P' {
			l.advance()
			// Optional 'i' for binary prefixes
			if !l.isAtEnd() && (l.peek() == 'i' || l.peek() == 'I') {
				l.advance()
			}
		}
	}

	value := l.input[start:l.pos]
	l.addToken(NUMERIC_LITERAL, value)
	return nil
}

// scanIdentifier scans an identifier or keyword
func (l *Lexer) scanIdentifier() error {
	start := l.pos

	// First character must be letter or underscore
	if !unicode.IsLetter(l.peek()) && l.peek() != '_' {
		return &ParseError{
			Message:  "Invalid identifier start",
			Position: l.start,
		}
	}

	l.advance() // consume first character

	// Continue with letters, digits, underscore, dot, or slash
	for !l.isAtEnd() {
		c := l.peek()
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '.' || c == '/' {
			l.advance()
		} else if c == '[' {
			// Handle array access syntax
			l.advance() // consume '['
			// Scan array index/key until ']'
			for !l.isAtEnd() && l.peek() != ']' {
				l.advance()
			}
			if !l.isAtEnd() {
				l.advance() // consume ']'
			}
		} else {
			break
		}
	}

	value := l.input[start:l.pos]

	// Check if it's a keyword (case-insensitive)
	lowerValue := strings.ToLower(value)
	if tokenType, isKeyword := keywords[lowerValue]; isKeyword {
		l.addToken(tokenType, value)
	} else {
		l.addToken(IDENTIFIER, value)
	}

	return nil
}

// NextToken returns the next token for the parser
func (l *Lexer) NextToken() Token {
	if l.current >= len(l.tokens) {
		return Token{Type: EOF, Value: "", Position: len(l.input)}
	}
	token := l.tokens[l.current]
	l.current++
	return token
}

// Peek returns the current token without advancing
func (l *Lexer) PeekToken() Token {
	if l.current >= len(l.tokens) {
		return Token{Type: EOF, Value: "", Position: len(l.input)}
	}
	return l.tokens[l.current]
}
