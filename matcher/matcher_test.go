package matcher_test

import (
	"testing"

	"github.com/CivNode/go-kata-grader/dsl"
	"github.com/CivNode/go-kata-grader/matcher"
)

func mustParse(t *testing.T, s string) *dsl.Node {
	t.Helper()
	n, err := dsl.Parse(s)
	if err != nil {
		t.Fatalf("dsl parse %q: %v", s, err)
	}
	return n
}

func match(t *testing.T, shape string, src string) bool {
	t.Helper()
	n := mustParse(t, shape)
	ok, err := matcher.Match(n, []byte(src))
	if err != nil {
		t.Fatalf("matcher: %v", err)
	}
	return ok
}

func TestMatch_CallContextWithTimeout(t *testing.T) {
	src := `package x
import "context"
import "time"
func F() {
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    _ = ctx
    _ = cancel
}`
	if !match(t, `call("context.WithTimeout", args=[_, _])`, src) {
		t.Fatal("expected match")
	}
}

func TestMatch_CallMissing(t *testing.T) {
	src := `package x
func F() { _ = 1 }`
	if match(t, `call("context.WithTimeout", args=[_, _])`, src) {
		t.Fatal("unexpected match")
	}
}

func TestMatch_ErrorWrap(t *testing.T) {
	src := `package x
import "fmt"
import "errors"
func F() error {
    base := errors.New("base")
    return fmt.Errorf("%w: wrapped", base)
}`
	if !match(t, `call("fmt.Errorf", args=[literal("%w", _), _])`, src) {
		t.Fatal("expected wrap match")
	}
}

func TestMatch_ErrorWrap_MissingVerb(t *testing.T) {
	src := `package x
import "fmt"
func F() error { return fmt.Errorf("plain: %s", "x") }`
	if match(t, `call("fmt.Errorf", args=[literal("%w", _), _])`, src) {
		t.Fatal("unexpected match")
	}
}

func TestMatch_AssignShape(t *testing.T) {
	src := `package x
import "context"
import "time"
func F() {
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    _, _ = ctx, cancel
}`
	if !match(t, `assign(lhs=[_, _], rhs=[call("context.WithTimeout", _)])`, src) {
		t.Fatal("expected assign match")
	}
}

func TestMatch_AssignWrongArity(t *testing.T) {
	src := `package x
import "context"
import "time"
func F() {
    ctx, cancel, extra := three()
    _, _, _ = ctx, cancel, extra
}
func three() (int, int, int) { return 1, 2, 3 }`
	if match(t, `assign(lhs=[_, _], rhs=[call("context.WithTimeout", _)])`, src) {
		t.Fatal("unexpected match")
	}
}

func TestMatch_CallUnorderedArgs(t *testing.T) {
	src := `package x
func g(a, b int) {}
func F() { g(2, 1) }`
	// Without unordered: 1,2 in wildcards still match because both are
	// wildcards; unordered is about value-bearing args. Use a more specific
	// shape to exercise it.
	// Here we assert that literal("foo", _) unordered also matches literal
	// appearing second.
	shape := mustParse(t, `call("h", args=[literal("foo", _), _], unordered)`)
	src2 := `package x
import "fmt"
func h(a int, s string) {}
func F() { h(42, "foo bar") }`
	file, err := matcher.Parse([]byte(src2))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !matcher.MatchFile(shape, file) {
		t.Fatal("expected unordered literal match")
	}
	_ = src
}

func TestMatch_BareCallByUnqualifiedName(t *testing.T) {
	// When the shape names a package-qualified function but the submission
	// uses a dot-imported bare call, we still match.
	src := `package x
import . "context"
func F() { WithTimeout(nil, 0) }`
	if !match(t, `call("context.WithTimeout", args=[_, _])`, src) {
		t.Fatal("expected bare-call match via dot-import fallback")
	}
}

func TestMatch_IdentExact(t *testing.T) {
	src := `package x
func F() { err := 1; _ = err }`
	if !match(t, `ident("err")`, src) {
		t.Fatal("expected ident match")
	}
	if match(t, `ident("nope")`, src) {
		t.Fatal("unexpected ident match")
	}
}

func TestMatch_CallNoArgsConstraint(t *testing.T) {
	src := `package x
func F() { make([]int, 10) }`
	if !match(t, `call("make")`, src) {
		t.Fatal("expected match without args")
	}
}

func TestMatch_NilShape(t *testing.T) {
	if matcher.MatchFile(nil, nil) {
		t.Fatal("nil shape should not match")
	}
}
