// Package matcher walks a Go AST and reports whether a DSL Node shape
// matches anywhere in it.
//
// Matches are identifier-rename-tolerant: a local binding named "result" in
// the submission is fine when the reference solution wrote "r". Matches are
// NOT tolerant of package-qualified renames: "context.WithTimeout" is
// matched only when the call site writes exactly "context.WithTimeout" (or
// "WithTimeout" as a bare name, falling back when the matcher cannot
// resolve the qualifier).
//
// Argument order is enforced by default; rules can opt into order
// insensitivity with the DSL "unordered" flag on call(...).
package matcher
