package katagrader

import (
	"fmt"

	"github.com/CivNode/go-kata-grader/dsl"
	"github.com/CivNode/go-kata-grader/matcher"
	"github.com/CivNode/go-kata-grader/mistakes"
)

// Evaluate analyses submission against kata and combines the component
// sub-scores into a single Grade. It never executes submission code.
//
// The verb is Evaluate rather than Grade to avoid a name collision with the
// Grade struct that carries the result. Callers reading "Grade" in the API
// surface see the noun-form type.
func Evaluate(submission []byte, kata Kata, testsPassed bool) (Grade, error) {
	return EvaluateWith(submission, kata, testsPassed, DefaultOptions())
}

// EvaluateWith is Evaluate with explicit Options so tests and integrators
// can inject real IdiomaticScorer and ConcurrencyAnalyzer implementations.
func EvaluateWith(submission []byte, kata Kata, testsPassed bool, opts Options) (Grade, error) {
	if opts.Idiomatic == nil {
		opts.Idiomatic = noopIdiomatic
	}
	if opts.Concurrency == nil {
		opts.Concurrency = noopConcurrency
	}

	g := Grade{
		PassedTests:    testsPassed,
		PatternMatches: map[string]bool{},
	}

	// Parse submission once; every component reuses the result when possible.
	file, err := matcher.Parse(submission)
	if err != nil {
		return g, fmt.Errorf("katagrader: parse submission: %w", err)
	}

	// Required patterns.
	for _, rule := range kata.RequiredPatterns {
		shape, parseErr := dsl.Parse(rule.ASTShape)
		if parseErr != nil {
			return g, fmt.Errorf("katagrader: required pattern %q: %w", rule.ID, parseErr)
		}
		matched := matcher.MatchFile(shape, file)
		g.PatternMatches[rule.ID] = matched
		if !matched {
			msg := rule.Message
			if msg == "" {
				msg = "no hint provided"
			}
			g.Notes = append(g.Notes, fmt.Sprintf("missing required pattern %q: %s", rule.ID, msg))
		}
	}

	// Forbidden patterns.
	for _, rule := range kata.ForbiddenPatterns {
		shape, parseErr := dsl.Parse(rule.ASTShape)
		if parseErr != nil {
			return g, fmt.Errorf("katagrader: forbidden pattern %q: %w", rule.ID, parseErr)
		}
		if matcher.MatchFile(shape, file) {
			g.ForbiddenHits = append(g.ForbiddenHits, rule.ID)
			msg := rule.Message
			if msg == "" {
				msg = "no hint provided"
			}
			g.Notes = append(g.Notes, fmt.Sprintf("forbidden pattern %q matched: %s", rule.ID, msg))
		}
	}

	// Mistake detectors.
	mistakeNotes, mErr := mistakes.Detect(submission)
	if mErr == nil {
		g.Notes = append(g.Notes, mistakeNotes...)
	} else {
		g.Notes = append(g.Notes, "mistake detectors unavailable: "+mErr.Error())
	}

	// Idiomaticness.
	score, idiomNotes, idiomOK := opts.Idiomatic(submission)
	if idiomOK {
		g.Idiomaticness = score
		g.Notes = append(g.Notes, idiomNotes...)
	} else {
		g.Idiomaticness = -1
		g.Notes = append(g.Notes, "idiomaticness score unavailable: go-idiomatic not wired up (tag v0.1.0 pending)")
	}

	// Concurrency safety.
	findings, concNotes, concOK := opts.Concurrency(submission)
	if concOK {
		g.ConcurrencySafe = findings == 0
		g.Notes = append(g.Notes, concNotes...)
	} else {
		g.ConcurrencySafe = true
		g.Notes = append(g.Notes, "concurrency analysis unavailable: go-concurrency-analysis not wired up (tag v0.1.0 pending); defaulting to safe")
	}

	g.Overall = computeOverall(g, len(kata.RequiredPatterns))
	return g, nil
}

// computeOverall applies the weighted formula from the Task 5 spec:
//
//	Overall = 0.4*passed + 0.3*idiomaticness + 0.15*concurrencySafe + 0.15*patternScore
//
// Each contributor is on a 0..100 scale; the output is rounded to a whole
// number. When Idiomaticness is unavailable we redistribute its weight
// evenly across the remaining contributors so a missing component does not
// punish the learner.
func computeOverall(g Grade, requiredCount int) int {
	passed := 0.0
	if g.PassedTests {
		passed = 100
	}
	concSafe := 0.0
	if g.ConcurrencySafe {
		concSafe = 100
	}

	patternScore := 100.0
	if requiredCount > 0 {
		matched := 0
		for _, ok := range g.PatternMatches {
			if ok {
				matched++
			}
		}
		patternScore = float64(matched) / float64(requiredCount) * 100
	}
	if len(g.ForbiddenHits) > 0 {
		// Every forbidden hit halves the pattern component, floored at 0.
		penalty := 1.0
		for range g.ForbiddenHits {
			penalty /= 2
		}
		patternScore *= penalty
	}

	weights := map[string]float64{
		"passed":  0.4,
		"idiom":   0.3,
		"conc":    0.15,
		"pattern": 0.15,
	}
	components := map[string]float64{
		"passed":  passed,
		"conc":    concSafe,
		"pattern": patternScore,
	}
	if g.Idiomaticness >= 0 {
		components["idiom"] = float64(g.Idiomaticness)
	} else {
		// Redistribute the idiom weight proportionally across the others.
		remaining := weights["passed"] + weights["conc"] + weights["pattern"]
		share := weights["idiom"] / remaining
		weights["passed"] *= 1 + share
		weights["conc"] *= 1 + share
		weights["pattern"] *= 1 + share
		delete(weights, "idiom")
	}

	sum := 0.0
	for name, w := range weights {
		if v, ok := components[name]; ok {
			sum += w * v
		}
	}
	// Round half-up.
	rounded := int(sum + 0.5)
	if rounded < 0 {
		rounded = 0
	}
	if rounded > 100 {
		rounded = 100
	}
	return rounded
}
