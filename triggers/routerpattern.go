// ============================================
// HELPER: Pattern matching for HTTP routing
// ============================================
package triggers

import (
	"regexp"
	"strings"
)

type RoutePattern struct {
	pattern *regexp.Regexp
	params  []string
}

func NewRoutePattern(path string) *RoutePattern {
	// Convert /api/users/:id to regex
	paramNames := []string{}
	regexPattern := path

	// Find all :param patterns
	paramRegex := regexp.MustCompile(`:(\w+)`)
	matches := paramRegex.FindAllStringSubmatch(path, -1)

	for _, match := range matches {
		paramNames = append(paramNames, match[1])
		regexPattern = strings.Replace(regexPattern, match[0], `([^/]+)`, 1)
	}

	// Compile regex
	pattern := regexp.MustCompile("^" + regexPattern + "$")

	return &RoutePattern{
		pattern: pattern,
		params:  paramNames,
	}
}

func (rp *RoutePattern) Match(path string) (map[string]string, bool) {
	matches := rp.pattern.FindStringSubmatch(path)
	if matches == nil {
		return nil, false
	}

	params := make(map[string]string)
	for i, name := range rp.params {
		params[name] = matches[i+1]
	}

	return params, true
}
