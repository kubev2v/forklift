package karl

import (
	corev1 "k8s.io/api/core/v1"
)

// NewKARLInterpreter creates a new KARL interpreter instance
func NewKARLInterpreter() *KARLInterpreter {
	return &KARLInterpreter{
		parser:    NewParser(),
		validator: NewValidator(),
		converter: NewConverter(),
	}
}

// Parse parses a single KARL rule from a string
func (k *KARLInterpreter) Parse(karlRule string) error {
	rule, err := k.parser.ParseRule(karlRule)
	if err != nil {
		return err
	}

	k.rule = rule
	return nil
}

// Validate validates the parsed rule
func (k *KARLInterpreter) Validate() error {
	return k.validator.ValidateRule(k.rule)
}

// ToAffinity converts the parsed KARL rule to Kubernetes Affinity
func (k *KARLInterpreter) ToAffinity() (*corev1.Affinity, error) {
	return k.converter.ToAffinity(k.rule)
}
