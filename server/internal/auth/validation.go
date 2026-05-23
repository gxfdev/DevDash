package auth

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	sqlInjectionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(\b(union\s+select|select\s+.+\s+from|insert\s+into|delete\s+from|drop\s+table|alter\s+table|truncate\s+table|exec\s*\(|execute\s*\(|xp_cmdshell|sp_executesql)\b)`),
		regexp.MustCompile(`(?i)(\b(or|and)\s+\d+\s*=\s*\d+)`),
		regexp.MustCompile(`(?i)(--|;|/\*|\*/|0x[0-9a-f]+)`),
		regexp.MustCompile(`(?i)('\s*(or|and)\s+')`),
		regexp.MustCompile(`(?i)(\bwaitfor\s+delay\b)`),
	}

	xssPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<\s*script[\s>]`),
		regexp.MustCompile(`(?i)javascript\s*:`),
		regexp.MustCompile(`(?i)vbscript\s*:`),
		regexp.MustCompile(`(?i)on(error|load|click|mouseover|focus|blur|submit|change|keyup|keydown)\s*=`),
		regexp.MustCompile(`(?i)<\s*(iframe|object|embed|form|input|textarea|button|link|meta|style|base)[\s>]`),
		regexp.MustCompile(`(?i)expression\s*\(`),
		regexp.MustCompile(`(?i)data\s*:\s*text/html`),
	}

	pathTraversalPattern = regexp.MustCompile(`\.\.[\\/]|[\\/]\.\.[\\/]`)
)

func ValidateInput(input string, maxLength int, fieldName string) error {
	if utf8.RuneCountInString(input) > maxLength {
		return fmt.Errorf("%s exceeds maximum length of %d", fieldName, maxLength)
	}
	if containsSQLInjection(input) {
		return fmt.Errorf("%s contains potentially dangerous content", fieldName)
	}
	if containsXSS(input) {
		return fmt.Errorf("%s contains potentially dangerous content", fieldName)
	}
	return nil
}

func containsSQLInjection(input string) bool {
	lower := strings.ToLower(input)
	for _, pattern := range sqlInjectionPatterns {
		if pattern.MatchString(lower) {
			return true
		}
	}
	return false
}

func containsXSS(input string) bool {
	for _, pattern := range xssPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

func SanitizeHTML(input string) string {
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, "\"", "&quot;")
	input = strings.ReplaceAll(input, "'", "&#x27;")
	input = strings.ReplaceAll(input, "`", "&#x60;")
	return input
}

func ValidatePath(path string) error {
	if pathTraversalPattern.MatchString(path) {
		return fmt.Errorf("path traversal detected")
	}
	return nil
}

func ValidateNodeID(id string) error {
	if len(id) == 0 || len(id) > 128 {
		return fmt.Errorf("invalid node id length")
	}
	for _, c := range id {
		if !isValidIDChar(c) {
			return fmt.Errorf("invalid character in node id")
		}
	}
	return nil
}

func isValidIDChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.'
}

func ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("username must be 3-32 characters")
	}
	for _, c := range username {
		if !isValidIDChar(c) {
			return fmt.Errorf("username contains invalid characters")
		}
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 128 {
		return fmt.Errorf("password must be at most 128 characters")
	}
	return nil
}

func ValidateSQLQuery(query string) error {
	dangerous := []string{"drop ", "alter ", "create ", "truncate ", "grant ", "revoke ", "exec ", "execute ", "xp_", "sp_", "0x"}
	lower := strings.ToLower(query)
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return fmt.Errorf("query contains forbidden keyword: %s", strings.TrimSpace(d))
		}
	}
	return nil
}
