package tsl

import "encoding/json"

// MarshalJSON implements json.Marshaler interface
func (n *TSLNode) MarshalJSON() ([]byte, error) {
	if n == nil {
		return []byte("null"), nil
	}

	type nodeAlias struct {
		Type  string      `json:"type"`
		Value interface{} `json:"value"`
	}

	// For binary and unary expressions, we need to handle the TSLExpressionOp specially
	if n.Type() == KindBinaryExpr || n.Type() == KindUnaryExpr {
		op := n.Value().(TSLExpressionOp)
		return json.Marshal(struct {
			Type     string      `json:"type"`
			Operator string      `json:"operator"`
			Left     *TSLNode    `json:"left,omitempty"`
			Right    *TSLNode    `json:"right,omitempty"`
			Value    interface{} `json:"value,omitempty"`
		}{
			Type:     n.Type().String(),
			Operator: op.Operator.String(),
			Left:     op.Left,
			Right:    op.Right,
		})
	}

	// For array literals, handle the array values
	if n.Type() == KindArrayLiteral {
		arr := n.Value().(TSLArrayLiteral)
		return json.Marshal(struct {
			Type   string     `json:"type"`
			Values []*TSLNode `json:"values"`
		}{
			Type:   n.Type().String(),
			Values: arr.Values,
		})
	}

	// For all other node types, use the default alias
	return json.Marshal(nodeAlias{
		Type:  n.Type().String(),
		Value: n.Value(),
	})
}

// MarshalYAML implements yaml.Marshaler interface
func (n *TSLNode) MarshalYAML() (interface{}, error) {
	if n == nil {
		return nil, nil
	}

	type nodeAlias struct {
		Type  string      `yaml:"type"`
		Value interface{} `yaml:"value"`
	}

	// For binary and unary expressions, we need to handle the TSLExpressionOp specially
	if n.Type() == KindBinaryExpr || n.Type() == KindUnaryExpr {
		op := n.Value().(TSLExpressionOp)
		return struct {
			Type     string      `yaml:"type"`
			Operator string      `yaml:"operator"`
			Left     *TSLNode    `yaml:"left,omitempty"`
			Right    *TSLNode    `yaml:"right,omitempty"`
			Value    interface{} `yaml:"value,omitempty"`
		}{
			Type:     n.Type().String(),
			Operator: op.Operator.String(),
			Left:     op.Left,
			Right:    op.Right,
		}, nil
	}

	// For array literals, handle the array values
	if n.Type() == KindArrayLiteral {
		arr := n.Value().(TSLArrayLiteral)
		return struct {
			Type   string     `yaml:"type"`
			Values []*TSLNode `yaml:"values"`
		}{
			Type:   n.Type().String(),
			Values: arr.Values,
		}, nil
	}

	// For all other node types, use the default alias
	return nodeAlias{
		Type:  n.Type().String(),
		Value: n.Value(),
	}, nil
}
