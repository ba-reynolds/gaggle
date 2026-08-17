package postutil

import (
	"regexp"
	"strings"
)

var hashtagPattern = regexp.MustCompile(`(?i)(?:^|[^\pL\pN_])#([\pL\pN_]{1,100})`)

func ExtractHashtags(content string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, match := range hashtagPattern.FindAllStringSubmatch(content, -1) {
		name := strings.ToLower(match[1])
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}
