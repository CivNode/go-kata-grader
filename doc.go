// Package katagrader grades Go code submissions against a kata's reference
// solution, required patterns, and forbidden patterns.
//
// Given a submission, a Kata description, and a pre-computed testsPassed
// signal, Evaluate returns a composite Grade comprising:
//
//   - Test pass/fail (PassedTests).
//   - Idiomaticness score via go-idiomatic (0..100).
//   - Concurrency safety via go-concurrency-analysis (bool).
//   - Required/forbidden pattern matches detected via the AST shape DSL.
//   - Common-mistake notes from the mistakes sub-package.
//   - An Overall weighted composite score (0..100).
//
// Evaluate never executes submission code. All analysis is AST-shape based
// and purely deterministic.
//
// The AST-shape DSL lives in the dsl sub-package; the matcher that runs
// parsed shapes against a Go AST lives in the matcher sub-package; and the
// common-mistake detectors live in mistakes.
package katagrader
