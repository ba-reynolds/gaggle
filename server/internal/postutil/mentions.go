package postutil

import (
	"regexp"
	"strings"
)

// mentionPattern matches @username tokens. Usernames are runs of unicode
// letters/digits/underscore up to 16 chars (users.username is VARCHAR(16)),
// and must be preceded by start-of-string or a non-word character so emails
// like "foo@bar.com" are not treated as mentions.
var mentionPattern = regexp.MustCompile(`(?:^|[^\pL\pN_])@([\pL\pN_]{1,16})`)

// ExtractMentions returns the unique usernames mentioned in content,
// normalized to lowercase. Callers must resolve them against the users table
// and drop unknowns.
func ExtractMentions(content string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, match := range mentionPattern.FindAllStringSubmatch(content, -1) {
		name := strings.ToLower(match[1])
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}