package ollantacore

// Violated reports whether actual satisfies the failing side of the comparison
// operator. relation must be one of "gt", "lt", "eq", "gte", or "lte".
func Violated(actual float64, relation string, threshold float64) bool {
	switch relation {
	case "gt":
		return actual > threshold
	case "lt":
		return actual < threshold
	case "eq":
		return actual == threshold
	case "gte":
		return actual >= threshold
	case "lte":
		return actual <= threshold
	}
	return false
}
