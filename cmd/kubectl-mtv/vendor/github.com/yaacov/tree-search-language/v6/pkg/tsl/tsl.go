// Copyright 2018 Yaacov Zamir <kobi.zamir@gmail.com>
// and other contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tsl

import (
	"github.com/yaacov/tree-search-language/v6/pkg/parser"
)

// TSLNode represents a tree search language AST (Abstract Syntax Tree) node
// This provides the public interface for TSL parsing results
type TSLNode struct {
	Node *Node
}

// TSLExpressionOp represents both unary and binary operations
type TSLExpressionOp struct {
	Operator    Operator
	Left, Right *TSLNode
}

// TSLArrayLiteral represents an array of nodes
type TSLArrayLiteral struct {
	Values []*TSLNode
}

// ParseTSL parses a TSL expression and returns the AST root node
func ParseTSL(input string) (*TSLNode, error) {
	parserNode, err := parser.Parse(input)
	if err != nil {
		// Return a TSL-specific error with position information
		if parseErr, ok := err.(*parser.ParseError); ok {
			return nil, &SyntaxError{
				Message:  parseErr.Message,
				Position: parseErr.Position,
				Context:  "",
				Input:    input,
			}
		}
		return nil, err
	}

	// Create TSL node from parsed input
	tslNode := wrapParserNode(parserNode)
	return &TSLNode{Node: tslNode}, nil
}

// Clone creates a deep copy of the TSLNode and its children
func (n *TSLNode) Clone() *TSLNode {
	if n == nil || n.Node == nil {
		return nil
	}
	return &TSLNode{Node: n.Node.Clone()}
}

// Type returns the type of the node
func (n *TSLNode) Type() Kind {
	if n == nil || n.Node == nil {
		return Kind(-1)
	}
	return n.Node.Kind
}

// Value returns the node's value based on its type
func (n *TSLNode) Value() interface{} {
	if n == nil || n.Node == nil {
		return nil
	}

	switch n.Node.Kind {
	case KindBooleanLiteral, KindNumericLiteral, KindStringLiteral,
		KindIdentifier, KindDateLiteral, KindTimestampLiteral:
		return n.Node.Value
	case KindBinaryExpr:
		var left, right *TSLNode
		if n.Node.Left != nil {
			left = &TSLNode{Node: n.Node.Left}
		}
		if n.Node.Right != nil {
			right = &TSLNode{Node: n.Node.Right}
		}
		return TSLExpressionOp{
			Operator: n.Node.Operator,
			Left:     left,
			Right:    right,
		}
	case KindUnaryExpr:
		var right *TSLNode
		if n.Node.Right != nil {
			right = &TSLNode{Node: n.Node.Right}
		}
		return TSLExpressionOp{
			Operator: n.Node.Operator,
			Left:     nil,
			Right:    right,
		}
	case KindArrayLiteral:
		values := make([]*TSLNode, len(n.Node.Children))
		for i, child := range n.Node.Children {
			values[i] = &TSLNode{Node: child}
		}
		return TSLArrayLiteral{Values: values}
	case KindNullLiteral:
		return "NULL"
	default:
		return nil
	}
}
