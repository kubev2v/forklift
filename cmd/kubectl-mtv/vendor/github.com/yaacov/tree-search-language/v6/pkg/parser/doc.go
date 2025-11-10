//go:generate goyacc -o parser.go parser.y

// Package parser provides the TSL (Tree Search Language) parser implementation.
//
// This package contains a pure Go implementation of the TSL parser with no
// external dependencies.
//
// The parser uses goyacc (Go port of yacc) and a custom Go lexer to parse
// TSL expressions into Abstract Syntax Tree (AST) nodes.
package parser
