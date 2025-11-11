package karl

import (
	corev1 "k8s.io/api/core/v1"
)

// RuleType represents the type of affinity rule
type RuleType string

const (
	RuleTypeRequire RuleType = "REQUIRE" // Hard affinity
	RuleTypePrefer  RuleType = "PREFER"  // Soft affinity
	RuleTypeAvoid   RuleType = "AVOID"   // Hard anti-affinity
	RuleTypeRepel   RuleType = "REPEL"   // Soft anti-affinity
)

// TopologyKey represents the topology domain
type TopologyKey string

const (
	TopologyNode   TopologyKey = "node"
	TopologyZone   TopologyKey = "zone"
	TopologyRegion TopologyKey = "region"
	TopologyRack   TopologyKey = "rack"
)

// LabelOperation represents the type of label operation
type LabelOperation string

const (
	LabelOpEquals    LabelOperation = "="
	LabelOpIn        LabelOperation = "in"
	LabelOpNotIn     LabelOperation = "not in"
	LabelOpExists    LabelOperation = "exists"
	LabelOpNotExists LabelOperation = "not exists"
)

// LabelSelector represents a single label selection criterion
type LabelSelector struct {
	Key       string
	Operation LabelOperation
	Values    []string // for equality, in, not in operations
}

// TargetSelector represents how to select target pods
type TargetSelector struct {
	Type           string          // pods
	LabelSelectors []LabelSelector // expressive label selectors
}

// KARLRule represents a parsed KARL rule
type KARLRule struct {
	RuleType       RuleType
	TargetSelector TargetSelector
	TopologyKey    TopologyKey
	Weight         int32 // for soft constraint rules
}

// KARLInterpreter handles parsing and conversion of KARL rules
type KARLInterpreter struct {
	rule      KARLRule
	parser    *Parser
	validator *Validator
	converter *Converter
}

// AffinityConverter interface for converting KARL rules to Kubernetes affinity
type AffinityConverter interface {
	ToAffinity(rule KARLRule) (*corev1.Affinity, error)
}

// RuleParser interface for parsing KARL rules
type RuleParser interface {
	ParseRule(karlRule string) (KARLRule, error)
}

// RuleValidator interface for validating KARL rules
type RuleValidator interface {
	ValidateRule(rule KARLRule) error
}
