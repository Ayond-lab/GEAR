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

type termSet struct {
	Canonical string
	Terms     []string
}

var protectedCriteria = []termSet{
	{Canonical: "nationality", Terms: []string{"nationality", "nationalities", "national"}},
	{Canonical: "citizenship", Terms: []string{"citizenship", "citizen", "citizens", "eea"}},
	{Canonical: "ethnicOrigin", Terms: []string{"ethnic origin", "ethnicity", "ethnic"}},
	{Canonical: "race", Terms: []string{"race", "racial"}},
	{Canonical: "religion", Terms: []string{"religion", "religious"}},
	{Canonical: "age", Terms: []string{"age", "aged"}},
	{Canonical: "sex", Terms: []string{"sex"}},
	{Canonical: "disability", Terms: []string{"disability", "disabled"}},
	{Canonical: "sexualOrientation", Terms: []string{"sexual orientation"}},
	{Canonical: "pregnancy", Terms: []string{"pregnancy", "pregnant"}},
	{Canonical: "maritalStatus", Terms: []string{"marital status", "married", "single"}},
	{Canonical: "tradeUnionMembership", Terms: []string{"trade union", "union membership"}},
}

var selectiveVerbs = []termSet{
	{Canonical: "select", Terms: []string{"select", "selects", "selected", "selecting"}},
	{Canonical: "rank", Terms: []string{"rank", "ranks", "ranked", "ranking"}},
	{Canonical: "filter", Terms: []string{"filter", "filters", "filtered", "filtering"}},
	{Canonical: "exclude", Terms: []string{"exclude", "excludes", "excluded", "excluding"}},
	{Canonical: "reject", Terms: []string{"reject", "rejects", "rejected", "rejecting"}},
	{Canonical: "shortlist", Terms: []string{"shortlist", "shortlists", "shortlisted", "shortlisting"}},
	{Canonical: "prioritise", Terms: []string{"prioritise", "prioritises", "prioritised", "prioritising", "prioritize", "prioritizes", "prioritized", "prioritizing"}},
	{Canonical: "score", Terms: []string{"score", "scores", "scored", "scoring"}},
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

func firstMatch(statement string, dictionary []termSet) string {
	lowered := strings.ToLower(statement)
	for _, set := range dictionary {
		for _, term := range set.Terms {
			if containsTerm(lowered, strings.ToLower(term)) {
				return set.Canonical
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
