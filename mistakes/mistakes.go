// Package mistakes hosts static detectors for common intermediate-Go
// mistakes that the grader surfaces as Notes.
//
// Each detector is pure: it takes parsed Go source and returns zero or more
// human-readable strings. The grader concatenates them into Grade.Notes.
package mistakes

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// Detect runs every built-in mistake detector against the source and
// returns their notes in a stable order.
func Detect(src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "submission.go", src, parser.AllErrors)
	if err != nil {
		// Try wrapping with a synthetic package clause so callers can pass
		// snippets that omit it.
		wrapped := append([]byte("package _sub\n"), src...)
		file, err = parser.ParseFile(fset, "submission.go", wrapped, parser.AllErrors)
		if err != nil {
			return nil, err
		}
	}
	var notes []string
	notes = append(notes, detectOffByOneIndex(fset, file)...)
	notes = append(notes, detectErrorSwallowing(fset, file)...)
	notes = append(notes, detectNilMapWrite(fset, file)...)
	notes = append(notes, detectDeferInLoop(fset, file)...)
	notes = append(notes, detectGoroutineWithoutSync(fset, file)...)
	return notes, nil
}

// detectOffByOneIndex flags classic `for i := 0; i <= len(x); i++` loops
// where `<=` should be `<`. The signal is a ForStmt whose Cond is a
// BinaryExpr with op LEQ and RHS is len(something).
func detectOffByOneIndex(fset *token.FileSet, file *ast.File) []string {
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		fs, ok := n.(*ast.ForStmt)
		if !ok || fs.Cond == nil {
			return true
		}
		be, ok := fs.Cond.(*ast.BinaryExpr)
		if !ok || be.Op != token.LEQ {
			return true
		}
		if !isLenCall(be.Y) {
			return true
		}
		out = append(out, fmt.Sprintf("off-by-one: %s uses i <= len(...); the idiomatic bound is <, not <=", posOf(fset, fs.Pos())))
		return true
	})
	return out
}

// detectErrorSwallowing flags assignments of the form `_, _ := f()` or `_, _ = f()`
// where f returns an error in at least one slot (heuristically: the RHS is a
// call and the assignment zero-discards every result).
func detectErrorSwallowing(fset *token.FileSet, file *ast.File) []string {
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if len(as.Lhs) < 2 {
			return true
		}
		allBlank := true
		for _, l := range as.Lhs {
			id, ok := l.(*ast.Ident)
			if !ok || id.Name != "_" {
				allBlank = false
				break
			}
		}
		if !allBlank {
			return true
		}
		if len(as.Rhs) != 1 {
			return true
		}
		if _, ok := as.Rhs[0].(*ast.CallExpr); !ok {
			return true
		}
		out = append(out, fmt.Sprintf("error swallowing: %s discards every result of a call; if one of them is an error, check it", posOf(fset, as.Pos())))
		return true
	})
	return out
}

// detectNilMapWrite flags the pattern
//
//	var m map[K]V
//	m[k] = v
//
// by finding assignments whose LHS is an IndexExpr on a variable that was
// declared via `var x map[...]...` without initialisation in the same
// function body.
func detectNilMapWrite(fset *token.FileSet, file *ast.File) []string {
	var out []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		nilMaps := map[string]token.Pos{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ds, ok := n.(*ast.DeclStmt)
			if !ok {
				return true
			}
			gd, ok := ds.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || vs.Type == nil || len(vs.Values) > 0 {
					continue
				}
				if _, isMap := vs.Type.(*ast.MapType); !isMap {
					continue
				}
				for _, name := range vs.Names {
					nilMaps[name.Name] = name.Pos()
				}
			}
			return true
		})
		if len(nilMaps) == 0 {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range as.Lhs {
				ix, ok := lhs.(*ast.IndexExpr)
				if !ok {
					continue
				}
				id, ok := ix.X.(*ast.Ident)
				if !ok {
					continue
				}
				if _, found := nilMaps[id.Name]; found {
					out = append(out, fmt.Sprintf("nil map write: %s assigns into %s which was declared with var but never make()'d", posOf(fset, as.Pos()), id.Name))
				}
			}
			return true
		})
	}
	return out
}

// detectDeferInLoop flags `defer` statements that appear inside `for` loop
// bodies (including `range` forms). This is almost always a bug since
// defers accumulate until function return.
func detectDeferInLoop(fset *token.FileSet, file *ast.File) []string {
	var out []string
	var walk func(node ast.Node, inLoop bool)
	walk = func(node ast.Node, inLoop bool) {
		if node == nil {
			return
		}
		switch n := node.(type) {
		case *ast.ForStmt:
			if n.Init != nil {
				walk(n.Init, inLoop)
			}
			if n.Cond != nil {
				walk(n.Cond, inLoop)
			}
			if n.Post != nil {
				walk(n.Post, inLoop)
			}
			walk(n.Body, true)
			return
		case *ast.RangeStmt:
			walk(n.Body, true)
			return
		case *ast.DeferStmt:
			if inLoop {
				out = append(out, fmt.Sprintf("defer in loop: %s defers inside a loop; defers accumulate until function return and rarely mean what the author hoped", posOf(fset, n.Pos())))
			}
			// Also walk the deferred call's args in case there are nested
			// loops inside them (rare).
			for _, a := range n.Call.Args {
				walk(a, inLoop)
			}
			return
		case *ast.FuncLit:
			// New function scope: defers here belong to the literal's frame.
			walk(n.Body, false)
			return
		case *ast.BlockStmt:
			for _, stmt := range n.List {
				walk(stmt, inLoop)
			}
			return
		case *ast.IfStmt:
			if n.Init != nil {
				walk(n.Init, inLoop)
			}
			walk(n.Body, inLoop)
			if n.Else != nil {
				walk(n.Else, inLoop)
			}
			return
		case *ast.SwitchStmt:
			if n.Init != nil {
				walk(n.Init, inLoop)
			}
			walk(n.Body, inLoop)
			return
		case *ast.TypeSwitchStmt:
			if n.Init != nil {
				walk(n.Init, inLoop)
			}
			walk(n.Body, inLoop)
			return
		case *ast.CaseClause:
			for _, stmt := range n.Body {
				walk(stmt, inLoop)
			}
			return
		case *ast.SelectStmt:
			walk(n.Body, inLoop)
			return
		case *ast.CommClause:
			for _, stmt := range n.Body {
				walk(stmt, inLoop)
			}
			return
		}
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			walk(fn.Body, false)
		}
	}
	return out
}

// detectGoroutineWithoutSync flags `go func(){ ... }()` calls in a function
// body where the surrounding scope does not reference sync.WaitGroup,
// sync.Mutex, channels, or context.Done. It is a coarse heuristic aimed at
// the classic "goroutine leak / forgotten sync" mistake.
func detectGoroutineWithoutSync(fset *token.FileSet, file *ast.File) []string {
	var out []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		bodySrc := nodeText(fset, fn.Body)
		hasSync := strings.Contains(bodySrc, "sync.") ||
			strings.Contains(bodySrc, "WaitGroup") ||
			strings.Contains(bodySrc, "<-") ||
			strings.Contains(bodySrc, "chan ") ||
			strings.Contains(bodySrc, "ctx.Done") ||
			strings.Contains(bodySrc, ".Done()")
		if hasSync {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			gs, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			out = append(out, fmt.Sprintf("goroutine without sync: %s launches a goroutine in %s but the enclosing function has no channel, WaitGroup, mutex, or context.Done to coordinate it", posOf(fset, gs.Pos()), fn.Name.Name))
			return true
		})
	}
	return out
}

// --- helpers ---------------------------------------------------------------

func isLenCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "len" && len(call.Args) == 1
}

func posOf(fset *token.FileSet, pos token.Pos) string {
	p := fset.Position(pos)
	if p.Filename == "" {
		return fmt.Sprintf("line %d", p.Line)
	}
	return fmt.Sprintf("%s:%d", p.Filename, p.Line)
}

func nodeText(fset *token.FileSet, n ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return ""
	}
	return buf.String()
}
