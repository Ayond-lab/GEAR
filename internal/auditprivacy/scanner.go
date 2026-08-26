package auditprivacy

import (
	"regexp"
	"strings"
)

type Finding struct {
	Pattern string
	Excerpt string
}

var defaultPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(candidate|applicant)\s+name\b`),
	regexp.MustCompile(`(?i)\bcv\s+text\b`),
	regexp.MustCompile(`(?i)\bfree[- ]text\s+reason\b`),
	regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`),
	regexp.MustCompile(`(?i)\b(phone|telephone)\b`),
	regexp.MustCompile(`(?i)\bsalt\b`),
}

func Scan(text string) []Finding {
	var findings []Finding
	for _, pattern := range defaultPatterns {
		loc := pattern.FindStringIndex(text)
		if loc == nil {
			continue
		}
		findings = append(findings, Finding{
			Pattern: pattern.String(),
			Excerpt: excerpt(text, loc[0], loc[1]),
		})
	}
	return findings
}

func excerpt(text string, start, end int) string {
	left := start - 24
	if left < 0 {
		left = 0
	}
	right := end + 24
	if right > len(text) {
		right = len(text)
	}
	return strings.TrimSpace(text[left:right])
}

