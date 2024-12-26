package fuego

import (
	"regexp"
	"strings"
)

var pathStdParamRegex = regexp.MustCompile(`{(.+?)}`)

// parsePathParams gives the list of path parameters in a path.
// Example : /item/{user}/{id} -> [user, id]
func parseStdPathParams(path string) []string {
	matches := pathStdParamRegex.FindAllString(path, -1)
	for i, match := range matches {
		matches[i] = strings.Trim(match, "{}")
	}
	return matches
}

func parseGinPathParams(path string) []string {
	params := []string{}

	for {
		idx := strings.IndexAny(path, "*:")
		if idx == -1 {
			break
		}

		var name string
		name, path, _ = strings.Cut(path[idx:], "/")
		params = append(params, name)
	}

	return params
}
