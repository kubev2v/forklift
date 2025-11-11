package karl

import (
	"fmt"
	"strings"
)

// Parser handles parsing of KARL rules and label selectors
type Parser struct{}

// NewParser creates a new parser instance
func NewParser() *Parser {
	return &Parser{}
}

// ParseRule parses a complete KARL rule string
func (p *Parser) ParseRule(karlRule string) (KARLRule, error) {
	rule := KARLRule{}

	// Clean up the rule
	line := strings.TrimSpace(karlRule)
	line = strings.TrimSuffix(line, ";")

	// Tokenize the rule
	tokens := p.tokenize(line)
	if len(tokens) < 4 {
		return rule, fmt.Errorf("invalid rule syntax: %s", line)
	}

	// Parse rule type
	ruleType, err := p.parseRuleType(tokens[0])
	if err != nil {
		return rule, err
	}
	rule.RuleType = ruleType

	// Parse target selector
	targetSelector, err := p.parseTargetSelector(tokens[1])
	if err != nil {
		return rule, fmt.Errorf("invalid target selector: %w", err)
	}
	rule.TargetSelector = targetSelector

	// Expect "on" keyword at index 2
	if strings.ToLower(tokens[2]) != "on" {
		return rule, fmt.Errorf("expected 'on' keyword, got: %s", tokens[2])
	}

	// Parse topology key (now at index 3)
	topologyKey, err := p.parseTopologyKey(tokens[3])
	if err != nil {
		return rule, err
	}
	rule.TopologyKey = topologyKey

	// Parse weight for soft constraint rules (PREFER and REPEL)
	if rule.RuleType == RuleTypePrefer || rule.RuleType == RuleTypeRepel {
		rule.Weight = 100 // default weight

		// Look for weight parameter (starting from index 4)
		for i := 4; i < len(tokens); i++ {
			if strings.HasPrefix(tokens[i], "weight=") {
				weight, err := p.parseWeight(tokens[i])
				if err != nil {
					return rule, err
				}
				rule.Weight = weight
				break
			}
		}
	}

	return rule, nil
}

// parseRuleType parses the rule type from a token
func (p *Parser) parseRuleType(token string) (RuleType, error) {
	switch strings.ToUpper(token) {
	case "REQUIRE":
		return RuleTypeRequire, nil
	case "PREFER":
		return RuleTypePrefer, nil
	case "AVOID":
		return RuleTypeAvoid, nil
	case "REPEL":
		return RuleTypeRepel, nil
	default:
		return "", fmt.Errorf("unknown rule type: %s", token)
	}
}

// parseTargetSelector parses target selector with expressive syntax
func (p *Parser) parseTargetSelector(selector string) (TargetSelector, error) {
	target := TargetSelector{}

	if !strings.Contains(selector, "(") || !strings.HasSuffix(selector, ")") {
		return target, fmt.Errorf("invalid selector format: %s", selector)
	}

	// Extract type and content
	parenIndex := strings.Index(selector, "(")
	selectorType := selector[:parenIndex]
	content := selector[parenIndex+1 : len(selector)-1]

	target.Type = selectorType

	switch selectorType {
	case "pods":
		// Parse expressive label selectors
		selectors, err := p.parseLabelSelectors(content)
		if err != nil {
			return target, fmt.Errorf("invalid label selector: %w", err)
		}
		target.LabelSelectors = selectors
	default:
		return target, fmt.Errorf("unsupported target type: %s", selectorType)
	}

	return target, nil
}

// parseTopologyKey parses the topology key from a token
func (p *Parser) parseTopologyKey(token string) (TopologyKey, error) {
	switch strings.ToLower(token) {
	case "node":
		return TopologyNode, nil
	case "zone":
		return TopologyZone, nil
	case "region":
		return TopologyRegion, nil
	case "rack":
		return TopologyRack, nil
	default:
		return "", fmt.Errorf("unknown topology key: %s", token)
	}
}

// parseWeight parses weight from a weight= token
func (p *Parser) parseWeight(token string) (int32, error) {
	weightStr := strings.TrimPrefix(token, "weight=")
	var weight int
	if _, err := fmt.Sscanf(weightStr, "%d", &weight); err != nil {
		return 0, fmt.Errorf("invalid weight value: %s", weightStr)
	}
	if weight < 1 || weight > 100 {
		return 0, fmt.Errorf("weight must be between 1 and 100: %d", weight)
	}
	return int32(weight), nil
}

// parseLabelSelectors parses expressive label selector syntax
func (p *Parser) parseLabelSelectors(content string) ([]LabelSelector, error) {
	var selectors []LabelSelector

	if content == "" {
		return selectors, nil
	}

	// Split by comma, but preserve content within brackets
	expressions := p.splitLabelExpressions(content)

	for _, expr := range expressions {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}

		selector, err := p.parseSingleLabelSelector(expr)
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, selector)
	}

	return selectors, nil
}

// splitLabelExpressions splits expressions by comma while preserving brackets
func (p *Parser) splitLabelExpressions(content string) []string {
	var expressions []string
	var current strings.Builder
	bracketDepth := 0

	for _, char := range content {
		switch char {
		case '[':
			bracketDepth++
			current.WriteRune(char)
		case ']':
			bracketDepth--
			current.WriteRune(char)
		case ',':
			if bracketDepth == 0 {
				expressions = append(expressions, current.String())
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		expressions = append(expressions, current.String())
	}

	return expressions
}

// parseSingleLabelSelector parses a single label selector expression
func (p *Parser) parseSingleLabelSelector(expr string) (LabelSelector, error) {
	expr = strings.TrimSpace(expr)

	// Handle "has key" syntax
	if strings.HasPrefix(expr, "has ") {
		key := strings.TrimSpace(expr[4:])
		return LabelSelector{
			Key:       key,
			Operation: LabelOpExists,
		}, nil
	}

	// Handle "not has key" syntax
	if strings.HasPrefix(expr, "not has ") {
		key := strings.TrimSpace(expr[8:])
		return LabelSelector{
			Key:       key,
			Operation: LabelOpNotExists,
		}, nil
	}

	// Handle "key not in [value1,value2]" syntax (must come before "in" check)
	if strings.Contains(expr, " not in [") && strings.HasSuffix(expr, "]") {
		parts := strings.Split(expr, " not in [")
		if len(parts) != 2 {
			return LabelSelector{}, fmt.Errorf("invalid 'not in' expression: %s", expr)
		}
		key := strings.TrimSpace(parts[0])
		valueList := strings.TrimSuffix(parts[1], "]")
		values := p.parseValueList(valueList)

		return LabelSelector{
			Key:       key,
			Operation: LabelOpNotIn,
			Values:    values,
		}, nil
	}

	// Handle "key in [value1,value2]" syntax
	if strings.Contains(expr, " in [") && strings.HasSuffix(expr, "]") {
		parts := strings.Split(expr, " in [")
		if len(parts) != 2 {
			return LabelSelector{}, fmt.Errorf("invalid 'in' expression: %s", expr)
		}
		key := strings.TrimSpace(parts[0])
		valueList := strings.TrimSuffix(parts[1], "]")
		values := p.parseValueList(valueList)

		return LabelSelector{
			Key:       key,
			Operation: LabelOpIn,
			Values:    values,
		}, nil
	}

	// Handle simple "key=value" syntax
	if strings.Contains(expr, "=") {
		kv := strings.Split(expr, "=")
		if len(kv) != 2 {
			return LabelSelector{}, fmt.Errorf("invalid equality expression: %s", expr)
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		return LabelSelector{
			Key:       key,
			Operation: LabelOpEquals,
			Values:    []string{value},
		}, nil
	}

	return LabelSelector{}, fmt.Errorf("unrecognized label selector expression: %s", expr)
}

// parseValueList parses a comma-separated list of values
func (p *Parser) parseValueList(valueList string) []string {
	var values []string
	parts := strings.Split(valueList, ",")
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

// tokenize splits a line into tokens while preserving quoted strings and parentheses
func (p *Parser) tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	parenDepth := 0

	for _, char := range line {
		switch char {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case '(':
			parenDepth++
			current.WriteRune(char)
		case ')':
			parenDepth--
			current.WriteRune(char)
		case ' ', '\t':
			if inQuotes || parenDepth > 0 {
				current.WriteRune(char)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
