package karl

import "fmt"

// Validator handles validation of KARL rules
type Validator struct{}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateRule validates a single KARL rule
func (v *Validator) ValidateRule(rule KARLRule) error {
	// Validate rule type
	if rule.RuleType == "" {
		return fmt.Errorf("missing rule type")
	}

	// Validate target selector
	if rule.TargetSelector.Type == "" {
		return fmt.Errorf("missing target selector")
	}

	if rule.TargetSelector.Type == "pods" && len(rule.TargetSelector.LabelSelectors) == 0 {
		return fmt.Errorf("pods selector requires labels")
	}

	// Validate label selectors
	if err := v.validateLabelSelectors(rule.TargetSelector.LabelSelectors); err != nil {
		return fmt.Errorf("invalid label selectors: %w", err)
	}

	// Validate topology key
	if rule.TopologyKey == "" {
		return fmt.Errorf("missing topology key")
	}

	// Validate weight for soft constraint rules
	if (rule.RuleType == RuleTypePrefer || rule.RuleType == RuleTypeRepel) && (rule.Weight < 1 || rule.Weight > 100) {
		return fmt.Errorf("soft constraint rule weight must be between 1 and 100")
	}

	// Validate target selector type
	if err := v.validateTargetSelector(rule.TargetSelector); err != nil {
		return fmt.Errorf("invalid target selector: %w", err)
	}

	return nil
}

// validateLabelSelectors validates individual label selectors
func (v *Validator) validateLabelSelectors(selectors []LabelSelector) error {
	for i, selector := range selectors {
		if err := v.validateLabelSelector(selector); err != nil {
			return fmt.Errorf("selector %d: %w", i, err)
		}
	}
	return nil
}

// validateLabelSelector validates a single label selector
func (v *Validator) validateLabelSelector(selector LabelSelector) error {
	// Validate key
	if selector.Key == "" {
		return fmt.Errorf("label key cannot be empty")
	}

	// Validate operation
	switch selector.Operation {
	case LabelOpEquals:
		if len(selector.Values) != 1 {
			return fmt.Errorf("equality operation requires exactly one value")
		}
		if selector.Values[0] == "" {
			return fmt.Errorf("equality operation value cannot be empty")
		}
	case LabelOpIn, LabelOpNotIn:
		if len(selector.Values) == 0 {
			return fmt.Errorf("%s operation requires at least one value", selector.Operation)
		}
		for i, value := range selector.Values {
			if value == "" {
				return fmt.Errorf("%s operation value %d cannot be empty", selector.Operation, i)
			}
		}
	case LabelOpExists, LabelOpNotExists:
		if len(selector.Values) != 0 {
			return fmt.Errorf("%s operation should not have values", selector.Operation)
		}
	default:
		return fmt.Errorf("unknown label operation: %s", selector.Operation)
	}

	return nil
}

// validateTargetSelector validates the target selector
func (v *Validator) validateTargetSelector(target TargetSelector) error {
	switch target.Type {
	case "pods":
		if len(target.LabelSelectors) == 0 {
			return fmt.Errorf("pods target requires at least one label selector")
		}
		return nil
	default:
		return fmt.Errorf("unsupported target type: %s", target.Type)
	}
}
