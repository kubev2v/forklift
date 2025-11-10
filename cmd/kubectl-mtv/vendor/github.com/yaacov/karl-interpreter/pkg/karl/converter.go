package karl

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Converter handles conversion of KARL rules to Kubernetes affinity
type Converter struct{}

// NewConverter creates a new converter instance
func NewConverter() *Converter {
	return &Converter{}
}

// ToAffinity converts a KARL rule to Kubernetes Affinity
func (c *Converter) ToAffinity(rule KARLRule) (*corev1.Affinity, error) {
	affinity := &corev1.Affinity{}

	if err := c.addRuleToAffinity(affinity, rule); err != nil {
		return nil, err
	}

	return affinity, nil
}

// addRuleToAffinity adds a single rule to the affinity structure
func (c *Converter) addRuleToAffinity(affinity *corev1.Affinity, rule KARLRule) error {
	// Get topology key
	topologyKey := c.getTopologyKey(rule)

	// Create label selector
	labelSelector, err := c.createLabelSelector(rule.TargetSelector)
	if err != nil {
		return err
	}

	// Determine if this is affinity or anti-affinity based on rule type
	isAntiAffinity := (rule.RuleType == RuleTypeAvoid || rule.RuleType == RuleTypeRepel)

	// Determine if this is a hard or soft constraint
	isHardConstraint := (rule.RuleType == RuleTypeRequire || rule.RuleType == RuleTypeAvoid)

	// Create pod affinity term
	if isHardConstraint {
		// Hard constraint
		term := corev1.PodAffinityTerm{
			LabelSelector: labelSelector,
			TopologyKey:   topologyKey,
		}

		if isAntiAffinity {
			if affinity.PodAntiAffinity == nil {
				affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
			}
			affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
				affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, term)
		} else {
			if affinity.PodAffinity == nil {
				affinity.PodAffinity = &corev1.PodAffinity{}
			}
			affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
				affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, term)
		}
	} else {
		// Soft constraint (PREFER and REPEL)
		weightedTerm := corev1.WeightedPodAffinityTerm{
			Weight: rule.Weight,
			PodAffinityTerm: corev1.PodAffinityTerm{
				LabelSelector: labelSelector,
				TopologyKey:   topologyKey,
			},
		}

		if isAntiAffinity {
			if affinity.PodAntiAffinity == nil {
				affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
			}
			affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, weightedTerm)
		} else {
			if affinity.PodAffinity == nil {
				affinity.PodAffinity = &corev1.PodAffinity{}
			}
			affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution, weightedTerm)
		}
	}

	return nil
}

// getTopologyKey converts KARL topology key to Kubernetes topology key
func (c *Converter) getTopologyKey(rule KARLRule) string {
	switch rule.TopologyKey {
	case TopologyNode:
		return "kubernetes.io/hostname"
	case TopologyZone:
		return "topology.kubernetes.io/zone"
	case TopologyRegion:
		return "topology.kubernetes.io/region"
	case TopologyRack:
		return "topology.kubernetes.io/rack"
	default:
		return "kubernetes.io/hostname" // fallback
	}
}

// createLabelSelector creates a Kubernetes label selector from target selector
func (c *Converter) createLabelSelector(target TargetSelector) (*metav1.LabelSelector, error) {
	if target.Type == "pods" {
		labelSelector := &metav1.LabelSelector{}

		// Separate different types of selectors
		matchLabels := make(map[string]string)
		var matchExpressions []metav1.LabelSelectorRequirement

		for _, selector := range target.LabelSelectors {
			switch selector.Operation {
			case LabelOpEquals:
				if len(selector.Values) > 0 {
					matchLabels[selector.Key] = selector.Values[0]
				}
			case LabelOpIn:
				req := metav1.LabelSelectorRequirement{
					Key:      selector.Key,
					Operator: metav1.LabelSelectorOpIn,
					Values:   selector.Values,
				}
				matchExpressions = append(matchExpressions, req)
			case LabelOpNotIn:
				req := metav1.LabelSelectorRequirement{
					Key:      selector.Key,
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   selector.Values,
				}
				matchExpressions = append(matchExpressions, req)
			case LabelOpExists:
				req := metav1.LabelSelectorRequirement{
					Key:      selector.Key,
					Operator: metav1.LabelSelectorOpExists,
				}
				matchExpressions = append(matchExpressions, req)
			case LabelOpNotExists:
				req := metav1.LabelSelectorRequirement{
					Key:      selector.Key,
					Operator: metav1.LabelSelectorOpDoesNotExist,
				}
				matchExpressions = append(matchExpressions, req)
			default:
				return nil, fmt.Errorf("unsupported label operation: %s", selector.Operation)
			}
		}

		if len(matchLabels) > 0 {
			labelSelector.MatchLabels = matchLabels
		}
		if len(matchExpressions) > 0 {
			labelSelector.MatchExpressions = matchExpressions
		}

		return labelSelector, nil
	}

	return &metav1.LabelSelector{}, fmt.Errorf("unsupported target type: %s", target.Type)
}
