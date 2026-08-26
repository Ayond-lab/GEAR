package legality

import (
	"regexp"
	"strings"
)

type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionRefuse Decision = "refuse"
)

type Evaluation struct {
	Decision     Decision
	Criterion    string
	Verb         string
	RuleID       string
	Reason       string
	Alternatives []string
}

var protectedCriteria = map[string][]string{
	"nationality":          {"nationality", "nationalities", "national"},
	"citizenship":          {"citizenship", "citizen", "citizens", "eea"},
	"ethnicOrigin":         {"ethnic origin", "ethnicity", "ethnic"},
	"race":                 {"race", "racial"},
	"religion":             {"religion", "religious"},
	"age":                  {"age", "aged"},
	"sex":                  {"sex"},
	"disability":           {"disability", "disabled"},
	"sexualOrientation":    {"sexual orientation"},
	"pregnancy":            {"pregnancy", "pregnant"},
	"maritalStatus":        {"marital status", "married", "single"},
	"tradeUnionMembership": {"trade union", "union membership"},
}

var selectiveVerbs = map[string][]string{
	"select":     {"select", "selects", "selected", "selecting"},
	"rank":       {"rank", "ranks", "ranked", "ranking"},
	"filter":     {"filter", "filters", "filtered", "filtering"},
	"exclude":    {"exclude", "excludes", "excluded", "excluding"},
	"reject":     {"reject", "rejects", "rejected", "rejecting"},
	"shortlist":  {"shortlist", "shortlists", "shortlisted", "shortlisting"},
	"prioritise": {"prioritise", "prioritises", "prioritised", "prioritising", "prioritize", "prioritizes", "prioritized", "prioritizing"},
	"score":      {"score", "scores", "scored", "scoring"},
}

var lawfulAlternatives = []string{
	"Record work-authorisation status for human planning without ranking, filtering, or excluding candidates.",
	"If a legal/security restriction applies, state the legal instrument as the mandate legal basis.",
}

func EvaluatePurpose(statement string) Evaluation {
	criterion := firstMatch(statement, protectedCriteria)
	verb := firstMatch(statement, selectiveVerbs)
	if criterion != "" && verb != "" {
		return Evaluation{
			Decision:     DecisionRefuse,
			Criterion:    criterion,
			Verb:         verb,
			RuleID:       "LG-PROTECTED-SELECTIVE-01",
			Reason:       "protected criterion combined with selective verb",
			Alternatives: append([]string(nil), lawfulAlternatives...),
		}
	}

	return Evaluation{Decision: DecisionAllow}
}

func firstMatch(statement string, dictionary map[string][]string) string {
	lowered := strings.ToLower(statement)
	for canonical, terms := range dictionary {
		for _, term := range terms {
			if containsTerm(lowered, strings.ToLower(term)) {
				return canonical
			}
		}
	}
	return ""
}

func containsTerm(statement, term string) bool {
	if strings.Contains(term, " ") {
		return strings.Contains(statement, term)
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(term) + `\b`)
	return re.MatchString(statement)
}

