package dsl_test

import (
	"testing"

	"github.com/CivNode/go-kata-grader/dsl"
)

func TestParse_Call(t *testing.T) {
	n, err := dsl.Parse(`call("context.WithTimeout", args=[_, _])`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Kind != "call" {
		t.Fatalf("kind: want call, got %q", n.Kind)
	}
	if n.Name != "context.WithTimeout" {
		t.Fatalf("name: %q", n.Name)
	}
	if len(n.Args) != 2 {
		t.Fatalf("args len: %d", len(n.Args))
	}
	for i, a := range n.Args {
		if a.Kind != "wildcard" {
			t.Fatalf("arg %d: kind %q", i, a.Kind)
		}
	}
}

func TestParse_NestedCallLiteral(t *testing.T) {
	n, err := dsl.Parse(`call("fmt.Errorf", args=[literal("%w", _)])`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Kind != "call" || n.Name != "fmt.Errorf" {
		t.Fatalf("top: %+v", n)
	}
	if len(n.Args) != 1 {
		t.Fatalf("args: %d", len(n.Args))
	}
	lit := n.Args[0]
	if lit.Kind != "literal" {
		t.Fatalf("inner kind: %q", lit.Kind)
	}
	if len(lit.Args) != 2 {
		t.Fatalf("literal args: %d", len(lit.Args))
	}
	if !lit.Args[0].HasString || lit.Args[0].StringLit != "%w" {
		t.Fatalf("first literal arg: %+v", lit.Args[0])
	}
	if lit.Args[1].Kind != "wildcard" {
		t.Fatalf("second literal arg kind: %q", lit.Args[1].Kind)
	}
}

func TestParse_AssignWithNested(t *testing.T) {
	n, err := dsl.Parse(`assign(lhs=[_, _], rhs=[call("context.WithTimeout", _)])`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Kind != "assign" {
		t.Fatalf("kind: %q", n.Kind)
	}
	if len(n.LHS) != 2 || len(n.RHS) != 1 {
		t.Fatalf("lhs/rhs lengths: %d/%d", len(n.LHS), len(n.RHS))
	}
	if n.RHS[0].Kind != "call" {
		t.Fatalf("rhs[0] kind: %q", n.RHS[0].Kind)
	}
}

func TestParse_Wildcard(t *testing.T) {
	n, err := dsl.Parse(`_`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Kind != "wildcard" {
		t.Fatalf("kind: %q", n.Kind)
	}
}

func TestParse_Ident(t *testing.T) {
	n, err := dsl.Parse(`ident("err")`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Kind != "ident" || n.Name != "err" {
		t.Fatalf("got %+v", n)
	}
}

func TestParse_CallUnordered(t *testing.T) {
	n, err := dsl.Parse(`call("sort.Slice", args=[_, _], unordered)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !n.Unordered {
		t.Fatalf("expected unordered")
	}
}

func TestParse_Errors(t *testing.T) {
	cases := []string{
		``,
		`call`,
		`call(`,
		`call(foo)`,
		`call("x" args=[])`,
		`call("x", args=[,])`,
		`call("x", args=)`,
		`unknown("x")`,
		`call("x", wat=1)`,
		`assign(lhs=, rhs=[])`,
		`assign(lhs=[])junk`,
		`ident()`,
	}
	for _, c := range cases {
		if _, err := dsl.Parse(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParse_LiteralBareString(t *testing.T) {
	// Bare string literal folds into a literal node.
	n, err := dsl.Parse(`"hello"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Kind != "literal" || !n.HasString || n.StringLit != "hello" {
		t.Fatalf("got %+v", n)
	}
}

func TestParse_EmptyArgs(t *testing.T) {
	n, err := dsl.Parse(`call("x", args=[])`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(n.Args) != 0 {
		t.Fatalf("args should be empty: %d", len(n.Args))
	}
}

func FuzzParse(f *testing.F) {
	seeds := []string{
		`call("context.WithTimeout", args=[_, _])`,
		`call("fmt.Errorf", args=[literal("%w", _)])`,
		`assign(lhs=[_, _], rhs=[call("context.WithTimeout", _)])`,
		`_`,
		`ident("err")`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = dsl.Parse(s)
	})
}
