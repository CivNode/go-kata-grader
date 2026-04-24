// Package dsl parses the AST-shape mini-DSL used by go-kata-grader's
// PatternRule. A DSL expression describes a shape to match somewhere in a
// submission's AST.
//
// Grammar (informal):
//
//	expr      := call | literal | assign | ident | wildcard
//	call      := "call" "(" string ("," "args" "=" list)? ("," "unordered")? ")"
//	literal   := "literal" "(" (arg ("," arg)*)? ")"
//	assign    := "assign" "(" ("lhs" "=" list)? ("," "rhs" "=" list)? ")"
//	ident     := "ident" "(" string ")"
//	list      := "[" (expr ("," expr)*)? "]"
//	wildcard  := "_"
//	arg       := expr | stringLit | numberLit
//
// Examples:
//
//	call("context.WithTimeout", args=[_, _])
//	call("fmt.Errorf", args=[literal("%w", _)])
//	assign(lhs=[_, _], rhs=[call("context.WithTimeout", _)])
//	call("sort.Slice", args=[_, _], unordered)
//	ident("err")
//
// Wildcards match any single sub-node. Identifier strings inside call and
// ident match exact names (with a single dot tolerated for selector form, so
// "context.WithTimeout" matches a SelectorExpr with X="context" and
// Sel="WithTimeout").
//
// Parse is the main entry point.
package dsl
