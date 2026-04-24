package matcher

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/CivNode/go-kata-grader/dsl"
)

// Match reports whether the parsed DSL shape matches anywhere in the given
// Go source.
func Match(shape *dsl.Node, src []byte) (bool, error) {
	file, err := parseSrc(src)
	if err != nil {
		return false, err
	}
	return MatchFile(shape, file), nil
}

// MatchFile reports whether the parsed DSL shape matches anywhere in the
// given pre-parsed Go file.
func MatchFile(shape *dsl.Node, file *ast.File) bool {
	if shape == nil || file == nil {
		return false
	}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		if matchNode(shape, n) {
			found = true
			return false
		}
		return true
	})
	return found
}

// Parse parses Go source into a *ast.File so callers can reuse the parse
// across many Match calls.
func Parse(src []byte) (*ast.File, error) {
	return parseSrc(src)
}

func parseSrc(src []byte) (*ast.File, error) {
	fset := token.NewFileSet()
	// Allow fragments that omit a package clause by trying a wrapper first
	// when the naked parse fails.
	file, err := parser.ParseFile(fset, "submission.go", src, parser.AllErrors)
	if err == nil {
		return file, nil
	}
	wrapped := append([]byte("package _sub\n"), src...)
	file2, err2 := parser.ParseFile(fset, "submission.go", wrapped, parser.AllErrors)
	if err2 == nil {
		return file2, nil
	}
	return nil, err
}

// matchNode dispatches on the DSL node kind and compares against the AST
// node. The AST node may or may not be the "natural" counterpart; we let
// each kind decide how strict to be.
func matchNode(shape *dsl.Node, n ast.Node) bool {
	if shape == nil || n == nil {
		return false
	}
	switch shape.Kind {
	case "wildcard":
		return true
	case "named":
		id, ok := n.(*ast.Ident)
		return ok && id.Name != "_"
	case "call":
		return matchCall(shape, n)
	case "literal":
		return matchLiteral(shape, n)
	case "assign":
		return matchAssign(shape, n)
	case "ident":
		return matchIdent(shape, n)
	default:
		return false
	}
}

func matchCall(shape *dsl.Node, n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	if !matchCallee(shape.Name, call.Fun) {
		return false
	}
	if shape.Args == nil {
		// No args constraint: the shape only cared about the callee.
		return true
	}
	if len(shape.Args) != len(call.Args) {
		return false
	}
	if shape.Unordered {
		return matchUnordered(shape.Args, call.Args)
	}
	for i, a := range shape.Args {
		if !matchExpr(a, call.Args[i]) {
			return false
		}
	}
	return true
}

func matchCallee(name string, fun ast.Expr) bool {
	if name == "" {
		return true
	}
	dot := strings.LastIndex(name, ".")
	switch f := fun.(type) {
	case *ast.Ident:
		if dot == -1 {
			return f.Name == name
		}
		// Package-qualified shape against bare identifier: accept only when
		// the bare name matches the unqualified part. This handles dot
		// imports in tests.
		return f.Name == name[dot+1:]
	case *ast.SelectorExpr:
		if dot == -1 {
			return f.Sel.Name == name
		}
		xIdent, ok := f.X.(*ast.Ident)
		if !ok {
			return false
		}
		return xIdent.Name == name[:dot] && f.Sel.Name == name[dot+1:]
	}
	return false
}

func matchUnordered(want []*dsl.Node, got []ast.Expr) bool {
	used := make([]bool, len(got))
	for _, w := range want {
		matched := false
		for j, g := range got {
			if used[j] {
				continue
			}
			if matchExpr(w, g) {
				used[j] = true
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func matchLiteral(shape *dsl.Node, n ast.Node) bool {
	// A bare string/number literal in the DSL matches a BasicLit whose
	// (unquoted) value equals the DSL string.
	if shape.HasString && len(shape.Args) == 0 {
		bl, ok := n.(*ast.BasicLit)
		if !ok {
			return false
		}
		return basicLitValue(bl) == shape.StringLit
	}
	// literal(a, b, c) is a positional tuple; at AST level we interpret it
	// as: this node is a sequence-ish position where consecutive Args match
	// consecutive sub-expressions. We currently use it inside Args lists of
	// call() to match CallExpr like fmt.Errorf("%w: ...", err). When the
	// AST node is a CallExpr whose first N args align with shape.Args, match.
	if call, ok := n.(*ast.CallExpr); ok {
		if len(shape.Args) > len(call.Args) {
			return false
		}
		for i, a := range shape.Args {
			if !matchExpr(a, call.Args[i]) {
				return false
			}
		}
		return true
	}
	// Alternative: when placed as the sole arg of a parent call, literal(x,
	// y) is handled by matchExpr which looks for a format-and-rest pattern
	// in a BasicLit + sibling. For a standalone AST node, treat as a single
	// BasicLit with leading format string.
	if bl, ok := n.(*ast.BasicLit); ok && len(shape.Args) > 0 {
		first := shape.Args[0]
		return first.HasString && basicLitContains(bl, first.StringLit)
	}
	return false
}

func matchAssign(shape *dsl.Node, n ast.Node) bool {
	a, ok := n.(*ast.AssignStmt)
	if !ok {
		return false
	}
	if shape.LHS != nil {
		if len(shape.LHS) != len(a.Lhs) {
			return false
		}
		for i, l := range shape.LHS {
			if !matchExpr(l, a.Lhs[i]) {
				return false
			}
		}
	}
	if shape.RHS != nil {
		if len(shape.RHS) != len(a.Rhs) {
			return false
		}
		for i, r := range shape.RHS {
			if !matchExpr(r, a.Rhs[i]) {
				return false
			}
		}
	}
	return true
}

func matchIdent(shape *dsl.Node, n ast.Node) bool {
	id, ok := n.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == shape.Name
}

// matchExpr is the entry point for matching a DSL node against an expression.
// It handles the Call/Literal/etc. dispatch plus the special case of literal
// appearing inside a call's args list.
func matchExpr(shape *dsl.Node, expr ast.Expr) bool {
	switch shape.Kind {
	case "wildcard":
		return true
	case "named":
		id, ok := expr.(*ast.Ident)
		return ok && id.Name != "_"
	case "ident":
		return matchIdent(shape, expr)
	case "call":
		return matchCall(shape, expr)
	case "literal":
		// literal("%w", _) inside a call's args list matches a BasicLit whose
		// text contains the shape's first string argument — i.e. we treat
		// literal() as a format-string recogniser. This covers the
		// fmt.Errorf("%w: wrap", err) use case.
		if shape.HasString && len(shape.Args) == 0 {
			bl, ok := expr.(*ast.BasicLit)
			if !ok {
				return false
			}
			return basicLitValue(bl) == shape.StringLit
		}
		if len(shape.Args) > 0 {
			first := shape.Args[0]
			if first.HasString {
				bl, ok := expr.(*ast.BasicLit)
				if !ok {
					return false
				}
				return basicLitContains(bl, first.StringLit)
			}
		}
		return false
	case "assign":
		return matchAssign(shape, expr)
	}
	return false
}

func basicLitValue(bl *ast.BasicLit) string {
	v := bl.Value
	if len(v) >= 2 && (v[0] == '"' || v[0] == '`') {
		return v[1 : len(v)-1]
	}
	return v
}

func basicLitContains(bl *ast.BasicLit, needle string) bool {
	return strings.Contains(basicLitValue(bl), needle)
}
