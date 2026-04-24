package katagrader_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	katagrader "github.com/CivNode/go-kata-grader"
)

type rulesFile struct {
	Required  []ruleJSON `json:"required"`
	Forbidden []ruleJSON `json:"forbidden"`
}

type ruleJSON struct {
	ID       string `json:"id"`
	ASTShape string `json:"astShape"`
	Message  string `json:"message"`
}

type wantFile struct {
	PassedTests           bool            `json:"passedTests"`
	ConcurrencySafe       bool            `json:"concurrencySafe"`
	PatternMatches        map[string]bool `json:"patternMatches"`
	ForbiddenHits         []string        `json:"forbiddenHits"`
	ExpectedNoteSubstring string          `json:"expectedNoteSubstring"`
	OverallAtLeast        int             `json:"overallAtLeast"`
	OverallAtMost         int             `json:"overallAtMost"`
}

func loadKata(t *testing.T, dir string) katagrader.Kata {
	t.Helper()
	ref, err := os.ReadFile(filepath.Join(dir, "reference.go"))
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	tests, err := os.ReadFile(filepath.Join(dir, "tests.go"))
	if err != nil {
		t.Fatalf("read tests: %v", err)
	}
	rulesRaw, err := os.ReadFile(filepath.Join(dir, "rules.json"))
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	var rules rulesFile
	if err := json.Unmarshal(rulesRaw, &rules); err != nil {
		t.Fatalf("parse rules: %v", err)
	}
	k := katagrader.Kata{
		ID:                filepath.Base(dir),
		ReferenceSolution: ref,
		TestsCode:         tests,
	}
	for _, r := range rules.Required {
		k.RequiredPatterns = append(k.RequiredPatterns, katagrader.PatternRule{
			ID: r.ID, ASTShape: r.ASTShape, Message: r.Message,
		})
	}
	for _, r := range rules.Forbidden {
		k.ForbiddenPatterns = append(k.ForbiddenPatterns, katagrader.PatternRule{
			ID: r.ID, ASTShape: r.ASTShape, Message: r.Message,
		})
	}
	return k
}

func TestGrade_ContextWithTimeoutKata(t *testing.T) {
	kataDir := "testdata/katas/context-with-timeout"
	kata := loadKata(t, kataDir)

	cases := []struct {
		name     string
		subFile  string
		wantFile string
	}{
		{"pass", "submissions/pass.go", "submissions/pass.want.json"},
		{"idiomatic_fail", "submissions/idiomatic_fail.go", "submissions/idiomatic_fail.want.json"},
		{"forbidden_hit", "submissions/forbidden_hit.go", "submissions/forbidden_hit.want.json"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sub, err := os.ReadFile(filepath.Join(kataDir, tc.subFile))
			if err != nil {
				t.Fatalf("read sub: %v", err)
			}
			wantRaw, err := os.ReadFile(filepath.Join(kataDir, tc.wantFile))
			if err != nil {
				t.Fatalf("read want: %v", err)
			}
			var want wantFile
			if err := json.Unmarshal(wantRaw, &want); err != nil {
				t.Fatalf("parse want: %v", err)
			}

			grade, err := katagrader.Evaluate(sub, kata, want.PassedTests)
			if err != nil {
				t.Fatalf("Grade: %v", err)
			}

			if grade.PassedTests != want.PassedTests {
				t.Errorf("PassedTests: got %v want %v", grade.PassedTests, want.PassedTests)
			}
			if grade.ConcurrencySafe != want.ConcurrencySafe {
				t.Errorf("ConcurrencySafe: got %v want %v", grade.ConcurrencySafe, want.ConcurrencySafe)
			}
			for id, exp := range want.PatternMatches {
				got := grade.PatternMatches[id]
				if got != exp {
					t.Errorf("PatternMatches[%s]: got %v want %v", id, got, exp)
				}
			}
			if !stringSliceEqual(grade.ForbiddenHits, want.ForbiddenHits) {
				t.Errorf("ForbiddenHits: got %v want %v", grade.ForbiddenHits, want.ForbiddenHits)
			}
			if want.ExpectedNoteSubstring != "" && !anyContains(grade.Notes, want.ExpectedNoteSubstring) {
				t.Errorf("Notes missing substring %q: %v", want.ExpectedNoteSubstring, grade.Notes)
			}
			if want.OverallAtLeast > 0 && grade.Overall < want.OverallAtLeast {
				t.Errorf("Overall: got %d want >= %d (notes=%v)", grade.Overall, want.OverallAtLeast, grade.Notes)
			}
			if want.OverallAtMost > 0 && grade.Overall > want.OverallAtMost {
				t.Errorf("Overall: got %d want <= %d (notes=%v)", grade.Overall, want.OverallAtMost, grade.Notes)
			}
		})
	}
}

func TestGrade_WithRealIdiomaticScorer(t *testing.T) {
	sub := []byte(`package x
import "context"
import "time"
func F(p context.Context, d time.Duration) (context.Context, context.CancelFunc) {
    return context.WithTimeout(p, d)
}`)
	opts := katagrader.Options{
		Idiomatic: func(_ []byte) (int, []string, bool) {
			return 92, nil, true
		},
		Concurrency: func(_ []byte) (int, []string, bool) {
			return 0, nil, true
		},
	}
	kata := katagrader.Kata{
		ID: "tiny",
		RequiredPatterns: []katagrader.PatternRule{{
			ID:       "uses-context-with-timeout",
			ASTShape: `call("context.WithTimeout", args=[_, _])`,
		}},
	}
	g, err := katagrader.EvaluateWith(sub, kata, true, opts)
	if err != nil {
		t.Fatalf("GradeWith: %v", err)
	}
	if g.Idiomaticness != 92 {
		t.Errorf("Idiomaticness: got %d want 92", g.Idiomaticness)
	}
	if !g.ConcurrencySafe {
		t.Error("expected concurrency safe")
	}
	if !g.PatternMatches["uses-context-with-timeout"] {
		t.Error("expected pattern match")
	}
	// With all four components at 100/92/100/100, overall should be near the
	// spec's weighted formula: 0.4*100 + 0.3*92 + 0.15*100 + 0.15*100 = 97.6.
	if g.Overall < 95 || g.Overall > 100 {
		t.Errorf("Overall: got %d want [95,100]", g.Overall)
	}
}

func TestGrade_DegradationNotesWhenAdaptersMissing(t *testing.T) {
	sub := []byte(`package x
func F() {}`)
	kata := katagrader.Kata{ID: "empty"}
	g, err := katagrader.Evaluate(sub, kata, true)
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if g.Idiomaticness != -1 {
		t.Errorf("expected Idiomaticness=-1 in stub mode, got %d", g.Idiomaticness)
	}
	if !g.ConcurrencySafe {
		t.Error("stub concurrency defaults to safe")
	}
	if !anyContains(g.Notes, "idiomaticness score unavailable") {
		t.Errorf("missing degradation note: %v", g.Notes)
	}
	if !anyContains(g.Notes, "concurrency analysis unavailable") {
		t.Errorf("missing concurrency degradation note: %v", g.Notes)
	}
}

func TestGrade_ParseFailure(t *testing.T) {
	_, err := katagrader.Evaluate([]byte("this is not go :::"), katagrader.Kata{}, false)
	if err == nil {
		t.Fatal("expected error on unparseable submission")
	}
}

func TestGrade_BadDSLInRule(t *testing.T) {
	sub := []byte(`package x
func F() {}`)
	kata := katagrader.Kata{
		RequiredPatterns: []katagrader.PatternRule{{
			ID:       "broken",
			ASTShape: `call(`,
		}},
	}
	_, err := katagrader.Evaluate(sub, kata, true)
	if err == nil {
		t.Fatal("expected error on bad DSL")
	}
}

func TestGrade_ForbiddenDetected(t *testing.T) {
	sub := []byte(`package x
import "time"
func F() { time.Sleep(time.Second) }`)
	kata := katagrader.Kata{
		ForbiddenPatterns: []katagrader.PatternRule{{
			ID:       "no-sleep",
			ASTShape: `call("time.Sleep", args=[_])`,
			Message:  "do not sleep",
		}},
	}
	g, err := katagrader.Evaluate(sub, kata, true)
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if len(g.ForbiddenHits) != 1 || g.ForbiddenHits[0] != "no-sleep" {
		t.Errorf("ForbiddenHits: %v", g.ForbiddenHits)
	}
	if !anyContains(g.Notes, "do not sleep") {
		t.Errorf("expected forbidden message: %v", g.Notes)
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func anyContains(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
