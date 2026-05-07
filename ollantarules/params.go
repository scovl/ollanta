package ollantarules

import "strconv"

// ParamInt reads an integer parameter from the rule params map, falling back to
// defaultVal when the key is absent or the value is not a valid integer.
func ParamInt(params map[string]string, key string, defaultVal int) int {
	if v, ok := params[key]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
