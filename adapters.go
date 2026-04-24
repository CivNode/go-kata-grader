package katagrader

// IdiomaticScorer scores a submission for Go-idiomaticness on a 0..100
// scale. The production implementation is wired up to
// github.com/CivNode/go-idiomatic@v0.1.0; until that release is published
// the grader uses a stub that returns (-1, nil) so the component's absence
// surfaces as a Note on the Grade.
type IdiomaticScorer func(src []byte) (score int, notes []string, ok bool)

// ConcurrencyAnalyzer reports whether a submission is concurrency-safe by
// returning a findings count. Zero findings means safe. The production
// implementation is wired up to
// github.com/CivNode/go-concurrency-analysis@v0.1.0; until that release is
// published the grader uses a stub that reports "unavailable".
type ConcurrencyAnalyzer func(src []byte) (findings int, notes []string, ok bool)

// noopIdiomatic is the default scorer when the upstream package is not yet
// tagged. It returns ok=false so the grader surfaces a Note explaining the
// missing component.
func noopIdiomatic(_ []byte) (int, []string, bool) {
	return -1, nil, false
}

// noopConcurrency is the default analyzer when the upstream package is not
// yet tagged. It returns ok=false so the grader surfaces a Note.
func noopConcurrency(_ []byte) (int, []string, bool) {
	return 0, nil, false
}

// Options configures a Grade call. The zero value uses the stub adapters.
// Integrators (CivNode, tests) swap in real implementations via
// WithIdiomaticScorer / WithConcurrencyAnalyzer.
type Options struct {
	Idiomatic   IdiomaticScorer
	Concurrency ConcurrencyAnalyzer
}

// DefaultOptions returns a fresh Options with the stub adapters. A real
// deployment overrides these before calling GradeWith.
func DefaultOptions() Options {
	return Options{
		Idiomatic:   noopIdiomatic,
		Concurrency: noopConcurrency,
	}
}
