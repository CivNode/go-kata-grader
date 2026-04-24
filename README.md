# go-kata-grader

AST-shape diffing, pattern matching, and common-mistake detection for grading Go code submissions. Part of the CivNode Training semantic engine.

Given a learner submission, a kata (reference solution plus required and forbidden pattern rules), and a pre-computed testsPassed signal, the grader produces a composite score and a list of human-readable notes. All analysis is deterministic and AST-based; submission code is never executed.

## Public API

```go
import katagrader "github.com/CivNode/go-kata-grader"

kata := katagrader.Kata{
    ID:                "context-with-timeout",
    ReferenceSolution: referenceSrc,
    TestsCode:         testsSrc,
    RequiredPatterns: []katagrader.PatternRule{{
        ID:       "uses-context-with-timeout",
        ASTShape: `call("context.WithTimeout", args=[_, _])`,
        Message:  "derive the context with context.WithTimeout",
    }},
    ForbiddenPatterns: []katagrader.PatternRule{{
        ID:       "no-time-sleep-coordination",
        ASTShape: `call("time.Sleep", args=[_])`,
        Message:  "use the context's Done channel, not time.Sleep",
    }},
}

grade, err := katagrader.Evaluate(submissionSrc, kata, testsPassed)
```

`Evaluate` is the exported verb; `Grade` is the returned value type.

`EvaluateWith(submission, kata, testsPassed, opts)` takes an `Options` struct that lets integrators plug in live `IdiomaticScorer` and `ConcurrencyAnalyzer` adapters backed by the companion repos:

- `github.com/CivNode/go-idiomatic`
- `github.com/CivNode/go-concurrency-analysis`

Until both ship a `v0.1.0` tag, the grader's default behaviour degrades gracefully: the idiomaticness component returns -1 with a Note, and the concurrency component defaults to safe (true) with a Note.

## AST-shape DSL

The `dsl` sub-package parses a mini-language for describing AST shapes:

```
call("context.WithTimeout", args=[_, _])
call("fmt.Errorf", args=[literal("%w", _), _])
assign(lhs=[named, named], rhs=[call("context.WithTimeout", args=[_, _])])
call("sort.Slice", args=[_, _], unordered)
ident("err")
```

- `_` is a wildcard (matches any expression, including the blank identifier).
- `named` matches any identifier that is not the blank `_`.
- Qualified names in `call("pkg.Func", ...)` match `pkg.Func` selectors exactly; they also match bare `Func` when the submission dot-imports the package.
- `unordered` on a call relaxes the positional arg check.

## Common-mistake detectors

`mistakes.Detect(src)` runs every built-in detector and returns notes the grader concatenates into `Grade.Notes`:

- Off-by-one in index loops (`for i := 0; i <= len(x); i++`).
- Error swallowing (`_, _ := f()` or `_, _ = f()`).
- Nil-map write (`var m map[K]V; m[k] = v`).
- Defer in loop.
- Goroutine without sync (no channel, WaitGroup, mutex, or context.Done).

## Scoring

```
Overall = 0.4 * passedTests + 0.3 * idiomaticness + 0.15 * concurrencySafe + 0.15 * patternScore
```

Each contributor is on a 0..100 scale. `patternScore` is the ratio of matched required patterns, halved for every forbidden hit. When the idiomaticness component is unavailable its weight is redistributed proportionally across the others.

## Development

```
make test    # go test ./... -race -count=1
make lint    # gofumpt + golangci-lint
make fuzz    # individual fuzz targets (e.g. dsl.FuzzParse)
```

## Licence

Apache-2.0. See [LICENSE](./LICENSE).
