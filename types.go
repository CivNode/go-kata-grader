package katagrader

// Kata is a single training challenge: a reference solution, its tests, and
// the pattern rules that define what "correct" looks like beyond the tests
// themselves.
type Kata struct {
	// ID is a stable slug identifying the kata.
	ID string
	// ReferenceSolution is the canonical Go source that solves the kata.
	ReferenceSolution []byte
	// TestsCode is the Go test source that the grader's caller runs (out of
	// process, typically via Yaegi) to produce the testsPassed signal.
	TestsCode []byte
	// RequiredPatterns is the list of AST shapes every correct submission
	// must contain.
	RequiredPatterns []PatternRule
	// ForbiddenPatterns is the list of AST shapes a correct submission must
	// not contain.
	ForbiddenPatterns []PatternRule
}

// PatternRule pairs a pattern ID with an AST-shape DSL expression and a
// human-readable message.
type PatternRule struct {
	// ID is a stable identifier for the rule, surfaced in results.
	ID string
	// ASTShape is the DSL expression that describes the AST shape to match.
	// See the dsl sub-package for grammar.
	ASTShape string
	// Message is shown to the learner when the rule fires (for forbidden
	// patterns) or when it is missing (for required patterns).
	Message string
}

// Grade is the result of grading a submission.
type Grade struct {
	// PassedTests mirrors the testsPassed argument to Grade.
	PassedTests bool
	// Idiomaticness is the go-idiomatic score (0..100). It is -1 when the
	// component is unavailable; a corresponding Note explains why.
	Idiomaticness int
	// ConcurrencySafe is true when go-concurrency-analysis reports zero
	// findings. It is true (optimistic) when the component is unavailable;
	// a corresponding Note explains the caveat.
	ConcurrencySafe bool
	// PatternMatches maps every required PatternRule.ID to whether the
	// submission contains the matching shape.
	PatternMatches map[string]bool
	// ForbiddenHits is the list of forbidden PatternRule.IDs that the
	// submission did match.
	ForbiddenHits []string
	// Notes collects human-readable observations: missed requirements,
	// forbidden hits, mistake detections, and degradation explanations.
	Notes []string
	// Overall is the composite grade (0..100).
	Overall int
}
