# flow

A tiny, extensible text pipeline: about 400 lines of dependency-free Go code.

`flow` reads lines from stdin and sends them through an ordered chain of
named stages. Stages live in a registry, so a new step is added by
registering a function — the engine itself never changes.

```console
$ printf 'banana\nApple\nbanana\n' | go run ./flow -steps 'sort,uniq,upper'
APPLE
BANANA
```

## Usage

```console
$ printf '  alice \n  bob\n' | go run ./flow -steps 'trim,upper,prefix:- '
- ALICE
- BOB
```

```console
$ printf 'alice 30\nbob 25\n' | go run ./flow -steps 'field:2: ,sort,number:> '
1> 25
2> 30
```

```console
$ printf 'the quick brown fox jumps over the lazy dog\n' | go run ./flow -steps 'wrap:20'
the quick brown fox
jumps over the lazy
dog
```

List every registered stage:

```console
$ go run ./flow -list
```

Stage specs are comma-separated. The first colon separates the stage name
from its argument, and the argument is passed through untouched, so
`prefix:> ` keeps its trailing space when quoted. Because commas separate
stages, a stage argument cannot contain a comma.

## Built-in stages

| Stage | Argument | Effect |
| --- | --- | --- |
| `upper` / `lower` | — | Change case. |
| `trim` | — | Trim leading and trailing whitespace. |
| `reverse` | — | Reverse the runes of each line. |
| `prefix` / `suffix` | `TEXT` | Add text before or after each line. |
| `replace` | `OLD:NEW` | Replace every `OLD` with `NEW`; `NEW` may contain colons. |
| `number` | `[SEP]` | Prefix line numbers, default separator `": "`. |
| `grep` / `drop` | `TEXT` | Keep only lines containing `TEXT`, or remove them. |
| `field` | `N:SEP` | Keep 1-based field `N` after splitting on `SEP`. |
| `sort` / `rsort` | — | Sort lines ascending or descending. |
| `uniq` | — | Drop duplicate lines, keeping first-seen order. |
| `head` / `tail` | `N` | Keep the first or last `N` lines. |
| `wrap` | `N` | Wrap each line at `N` runes on spaces, without splitting words. |
| `join` | `SEP` | Join all lines into a single line. |
| `rot13` | — | Custom stage defined in `custom.go`. |

## Extending it

`custom.go` is the extension template. The `rot13` stage is registered from
`init`, so it exists for every pipeline without any engine change:

```go
func init() {
	Must(Default.Register("rot13", Simple(rot13Encode)))
}
```

A stage with an argument builds a `Stage` from the raw argument instead:

```go
func init() {
	Must(Default.Register("repeat", repeatStep))
}

func repeatStep(arg string) (Stage, error) {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("want repeat:N")
	}
	return PerLine(func(line string) (string, error) {
		return strings.Repeat(line, n), nil
	}), nil
}
```

`Simple` adapts an argument-free `func(string) string`; `PerLine` adapts a
single-line transform. A stage may also work on the whole stream of lines
(`sort`, `uniq`, `head`, `wrap` do), which is how one interface covers both
line filters and reordering.

Add the stage to `steps.go` or a new file, then run `go run ./flow -list` to
confirm it is registered.

## Design

- `Registry` maps stage names to `Definition` functions.
- `Compile` resolves a list of specs once; `Pipeline.Run` applies the stages.
- Duplicate or invalid registrations fail fast instead of silently replacing
  a stage.
- Errors include the step index and, for line stages, the failing line number.

## Tests

```console
$ go test ./...
```
