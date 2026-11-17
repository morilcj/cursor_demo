package main

import (
	"fmt"
	"strings"
	"testing"
)

func compile(t *testing.T, registry *Registry, specs ...string) *Pipeline {
	t.Helper()
	pipeline, err := registry.Compile(specs)
	if err != nil {
		t.Fatalf("Compile(%q): %v", specs, err)
	}
	return pipeline
}

func run(t *testing.T, registry *Registry, input string, specs ...string) string {
	t.Helper()
	output, err := compile(t, registry, specs...).Run(input)
	if err != nil {
		t.Fatalf("Run(%q): %v", specs, err)
	}
	return output
}

func TestBuiltinStages(t *testing.T) {
	tests := []struct {
		name  string
		input string
		specs []string
		want  string
	}{
		{"upper", "Hello\n", []string{"upper"}, "HELLO\n"},
		{"lower", "Hello\n", []string{"lower"}, "hello\n"},
		{"trim", "  padded  \n", []string{"trim"}, "padded\n"},
		{"reverse handles unicode", "héllo 世界\n", []string{"reverse"}, "界世 olléh\n"},
		{"prefix", "x\n", []string{"prefix:> "}, "> x\n"},
		{"suffix", "x\n", []string{"suffix:!"}, "x!\n"},
		{"replace", "cat catalog\n", []string{"replace:cat:dog"}, "dog dogalog\n"},
		{"replace keeps colons in the replacement", "a\n", []string{"replace:a:b:c"}, "b:c\n"},
		{"number defaults to colon", "a\nb\n", []string{"number"}, "1: a\n2: b\n"},
		{"number accepts a separator", "a\nb\n", []string{"number:> "}, "1> a\n2> b\n"},
		{"grep keeps matching lines", "apple\npear\napricot\n", []string{"grep:ap"}, "apple\napricot\n"},
		{"drop removes matching lines", "apple\npear\napricot\n", []string{"drop:ap"}, "pear\n"},
		{"field selects a column", "a:b:c\n", []string{"field:2::"}, "b\n"},
		{"field tolerates short lines", "a:b\n", []string{"field:3::"}, "\n"},
		{"sort", "pear\napple\n", []string{"sort"}, "apple\npear\n"},
		{"rsort", "apple\npear\n", []string{"rsort"}, "pear\napple\n"},
		{"uniq keeps first occurrence order", "b\na\nb\n", []string{"uniq"}, "b\na\n"},
		{"head", "a\nb\nc\n", []string{"head:2"}, "a\nb\n"},
		{"tail", "a\nb\nc\n", []string{"tail:2"}, "b\nc\n"},
		{"wrap", "one two three four\n", []string{"wrap:9"}, "one two\nthree\nfour\n"},
		{"wrap does not split long words", "abcdefghij klm\n", []string{"wrap:3"}, "abcdefghij\nklm\n"},
		{"join", "a\nb\nc\n", []string{"join:-"}, "a-b-c\n"},
		{"rot13 custom stage", "Hello, World!\n", []string{"rot13"}, "Uryyb, Jbeyq!\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, Default, tt.input, tt.specs...); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStageOrderMatters(t *testing.T) {
	got := run(t, Default, "  hello \n", "trim", "upper", "prefix:> ")
	if want := "> HELLO\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	got = run(t, Default, "b\na\n", "sort", "number")
	if want := "1: a\n2: b\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunPreservesNewlineShape(t *testing.T) {
	if got := run(t, Default, ""); got != "" {
		t.Errorf("empty input: got %q, want empty", got)
	}
	if got := run(t, Default, "a\nb", "upper"); got != "A\nB" {
		t.Errorf("missing trailing newline: got %q, want %q", got, "A\nB")
	}
	if got := run(t, Default, "a\n\nb\n", "upper"); got != "A\n\nB\n" {
		t.Errorf("blank line: got %q, want %q", got, "A\n\nB\n")
	}
	if got := run(t, Default, "\n", "upper"); got != "\n" {
		t.Errorf("single blank line: got %q, want %q", got, "\n")
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name  string
		specs []string
		want  string
	}{
		{"unknown step", []string{"nope"}, "unknown step"},
		{"empty step", []string{"upper", " "}, "empty step"},
		{"upper takes no argument", []string{"upper:oops"}, "takes no argument"},
		{"sort takes no argument", []string{"sort:desc"}, "takes no argument"},
		{"prefix without text", []string{"prefix"}, "want prefix:TEXT"},
		{"suffix without text", []string{"suffix"}, "want suffix:TEXT"},
		{"replace without separator", []string{"replace:old"}, "want replace:OLD:NEW"},
		{"replace without old", []string{"replace::new"}, "want replace:OLD:NEW"},
		{"grep without text", []string{"grep"}, "want grep:TEXT"},
		{"field index", []string{"field:x::"}, "field index"},
		{"field no separator", []string{"field:2"}, "want field:N:SEPARATOR"},
		{"head count", []string{"head:nope"}, "non-negative integer"},
		{"wrap zero", []string{"wrap:0"}, "wrap width must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Default.Compile(tt.specs)
			if err == nil {
				t.Fatalf("Compile(%q) succeeded, want error containing %q", tt.specs, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Compile(%q) error = %q, want substring %q", tt.specs, err, tt.want)
			}
		})
	}
}

func TestCompileTrimsNameButNotArgument(t *testing.T) {
	got := run(t, Default, "x\n", " prefix:> ")
	if want := "> x\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRegisterRejectsBadDefinitions(t *testing.T) {
	registry := NewRegistry()
	identity := Simple(func(s string) string { return s })

	if err := registry.Register("", identity); err == nil {
		t.Error("empty name was accepted")
	}
	if err := registry.Register("has space", identity); err == nil {
		t.Error("name with a space was accepted")
	}
	if err := registry.Register("colon:name", identity); err == nil {
		t.Error("name with a colon was accepted")
	}
	if err := registry.Register("nil", nil); err == nil {
		t.Error("nil definition was accepted")
	}
	if err := registry.Register("ok", identity); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := registry.Register("ok", identity); err == nil {
		t.Error("duplicate registration was accepted")
	}
}

func TestNamesAreSorted(t *testing.T) {
	names := Default.Names()
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("names are not sorted: %q", names)
		}
	}
	if len(names) == 0 {
		t.Fatal("no stages registered")
	}
}

func TestStageErrorIsWrapped(t *testing.T) {
	registry := NewRegistry()
	Must(registry.Register("boom", func(string) (Stage, error) {
		return PerLine(func(line string) (string, error) {
			if line == "bad" {
				return "", fmt.Errorf("kaboom")
			}
			return line, nil
		}), nil
	}))
	pipeline := compile(t, registry, "boom")
	_, err := pipeline.Run("fine\nbad\n")
	if err == nil {
		t.Fatal("Run succeeded, want an error")
	}
	for _, want := range []string{"step 1", "boom", "line 2", "kaboom"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestNewStageIsImmediatelyAvailable(t *testing.T) {
	registry := NewRegistry()
	Must(registry.Register("count", func(arg string) (Stage, error) {
		return func(lines []string) ([]string, error) {
			return []string{arg + fmt.Sprint(len(lines))}, nil
		}, nil
	}))
	got := run(t, registry, "a\nb\nc\n", "count:lines=")
	if want := "lines=3\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNumberStartsAtOnePerPipeline(t *testing.T) {
	first := run(t, Default, "a\nb\n", "number")
	second := run(t, Default, "c\n", "number")
	if want := "1: a\n2: b\n"; first != want {
		t.Errorf("first pipeline = %q, want %q", first, want)
	}
	if want := "1: c\n"; second != want {
		t.Errorf("second pipeline = %q, want %q", second, want)
	}
}

func TestNoStagesPassesInputThrough(t *testing.T) {
	got := run(t, Default, "a\nb\n")
	if want := "a\nb\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
