package mistakes_test

import (
	"strings"
	"testing"

	"github.com/CivNode/go-kata-grader/mistakes"
)

func runDetect(t *testing.T, src string) []string {
	t.Helper()
	notes, err := mistakes.Detect([]byte(src))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	return notes
}

func hasNote(notes []string, substr string) bool {
	for _, n := range notes {
		if strings.Contains(n, substr) {
			return true
		}
	}
	return false
}

func TestDetect_OffByOne(t *testing.T) {
	src := `package x
func F(xs []int) {
    for i := 0; i <= len(xs); i++ { _ = xs[i] }
}`
	notes := runDetect(t, src)
	if !hasNote(notes, "off-by-one") {
		t.Fatalf("missing off-by-one: %v", notes)
	}
}

func TestDetect_OffByOneNegative(t *testing.T) {
	src := `package x
func F(xs []int) {
    for i := 0; i < len(xs); i++ { _ = xs[i] }
}`
	notes := runDetect(t, src)
	if hasNote(notes, "off-by-one") {
		t.Fatalf("unexpected off-by-one: %v", notes)
	}
}

func TestDetect_ErrorSwallowing(t *testing.T) {
	src := `package x
import "os"
func F() {
    _, _ = os.ReadFile("x.txt")
}`
	notes := runDetect(t, src)
	if !hasNote(notes, "error swallowing") {
		t.Fatalf("missing error swallowing: %v", notes)
	}
}

func TestDetect_NilMapWrite(t *testing.T) {
	src := `package x
func F() {
    var m map[string]int
    m["a"] = 1
}`
	notes := runDetect(t, src)
	if !hasNote(notes, "nil map write") {
		t.Fatalf("missing nil map write: %v", notes)
	}
}

func TestDetect_NilMapWrite_NegativeWithMake(t *testing.T) {
	src := `package x
func F() {
    m := make(map[string]int)
    m["a"] = 1
}`
	notes := runDetect(t, src)
	if hasNote(notes, "nil map write") {
		t.Fatalf("unexpected nil map write: %v", notes)
	}
}

func TestDetect_DeferInLoop(t *testing.T) {
	src := `package x
import "os"
func F(paths []string) {
    for _, p := range paths {
        f, _ := os.Open(p)
        defer f.Close()
    }
}`
	notes := runDetect(t, src)
	if !hasNote(notes, "defer in loop") {
		t.Fatalf("missing defer in loop: %v", notes)
	}
}

func TestDetect_DeferInFuncLitInsideLoop_NotFlagged(t *testing.T) {
	src := `package x
import "os"
func F(paths []string) {
    for _, p := range paths {
        func() {
            f, _ := os.Open(p)
            defer f.Close()
        }()
    }
}`
	notes := runDetect(t, src)
	if hasNote(notes, "defer in loop") {
		t.Fatalf("unexpected defer-in-loop flag for func literal: %v", notes)
	}
}

func TestDetect_GoroutineWithoutSync(t *testing.T) {
	src := `package x
import "fmt"
func F() {
    go func() { fmt.Println("hi") }()
}`
	notes := runDetect(t, src)
	if !hasNote(notes, "goroutine without sync") {
		t.Fatalf("missing goroutine without sync: %v", notes)
	}
}

func TestDetect_GoroutineWithChannel_NotFlagged(t *testing.T) {
	src := `package x
func F() {
    done := make(chan struct{})
    go func() { done <- struct{}{} }()
    <-done
}`
	notes := runDetect(t, src)
	if hasNote(notes, "goroutine without sync") {
		t.Fatalf("unexpected goroutine-without-sync flag: %v", notes)
	}
}

func TestDetect_GoroutineWithWaitGroup_NotFlagged(t *testing.T) {
	src := `package x
import "sync"
func F() {
    var wg sync.WaitGroup
    wg.Add(1)
    go func() { defer wg.Done() }()
    wg.Wait()
}`
	notes := runDetect(t, src)
	if hasNote(notes, "goroutine without sync") {
		t.Fatalf("unexpected flag with WaitGroup: %v", notes)
	}
}

func TestDetect_ParseError(t *testing.T) {
	if _, err := mistakes.Detect([]byte("not valid go :::")); err == nil {
		t.Fatal("expected parse error")
	}
}
