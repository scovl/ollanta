// Package summarizer propagates metric values up a Component tree using hierarchical
// summation (cumSum). Inspired by the cumSum() algorithm in MetricSum.h from
// OpenStaticAnalyzer, which accumulates metrics along containment edges from leaves
// to the project root.
package summarizer

import "github.com/scovl/ollanta/ollantacore/domain"

// CumSum propagates each metric in metricKeys up the Component tree by summing child
// values into their parent. Leaf values are left unchanged; every ancestor receives the
// sum of all descendants below it.
func CumSum(root *domain.Component, metricKeys []string) {
	if root == nil {
		return
	}
	cumSumNode(root, metricKeys)
}

// cumSumNode recursively computes the cumulative sum for a node and returns its
// accumulated metric values so the parent can add them.
func cumSumNode(c *domain.Component, keys []string) map[string]float64 {
	if c.Metrics == nil {
		c.Metrics = map[string]float64{}
	}

	// Leaf node: return its own values as-is.
	if len(c.Children) == 0 {
		result := make(map[string]float64, len(keys))
		for _, k := range keys {
			result[k] = c.Metrics[k]
		}
		return result
	}

	// Non-leaf: accumulate from children.
	totals := make(map[string]float64, len(keys))
	for _, child := range c.Children {
		childTotals := cumSumNode(child, keys)
		for _, k := range keys {
			totals[k] += childTotals[k]
		}
	}
	for _, k := range keys {
		c.Metrics[k] = totals[k]
	}
	return totals
}

// CumAvg computes a hierarchical average at every non-leaf node:
//
//	avgKey = Metrics[totalKey] / Metrics[countKey]
//
// Leaf node values are left unchanged. If countKey is zero at a node, avgKey is set
// to 0 to avoid division by zero.
func CumAvg(root *domain.Component, totalKey, countKey, avgKey string) {
	if root == nil {
		return
	}
	cumAvgNode(root, totalKey, countKey, avgKey)
}

func cumAvgNode(c *domain.Component, totalKey, countKey, avgKey string) {
	if c.Metrics == nil {
		c.Metrics = map[string]float64{}
	}
	for _, child := range c.Children {
		cumAvgNode(child, totalKey, countKey, avgKey)
	}
	if len(c.Children) == 0 {
		return // leaf: leave avgKey as set by the caller
	}
	count := c.Metrics[countKey]
	if count == 0 {
		c.Metrics[avgKey] = 0
	} else {
		c.Metrics[avgKey] = c.Metrics[totalKey] / count
	}
}
