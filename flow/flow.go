// Package main implements flow, a tiny extensible text pipeline.
//
// A pipeline is an ordered list of named stages. Each stage transforms the
// whole stream of lines, so simple per-line steps and stream-wide steps
// (sort, uniq, wrap, ...) compose the same way. Stages live in a Registry:
// adding one never requires changing the engine.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// LineFunc transforms one line into one line. PerLine lifts a LineFunc into
// a Stage that applies it to the whole stream and reports which line failed,
// so simple stages such as upper do not have to write the loop themselves.
type LineFunc func(string) (string, error)

// Stage transforms a whole stream of lines.
type Stage func(lines []string) ([]string, error)

// Definition builds a Stage from the raw argument after "name:" in a spec.
type Definition func(arg string) (Stage, error)

// PerLine adapts a LineFunc into a Stage.
func PerLine(fn LineFunc) Stage {
	return func(lines []string) ([]string, error) {
		out := make([]string, len(lines))
		for i, line := range lines {
			transformed, err := fn(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			out[i] = transformed
		}
		return out, nil
	}
}

// Simple adapts an argument-free string function into a Definition.
func Simple(fn func(string) string) Definition {
	return func(arg string) (Stage, error) {
		if arg != "" {
			return nil, fmt.Errorf("takes no argument")
		}
		return PerLine(func(line string) (string, error) { return fn(line), nil }), nil
	}
}

// Registry holds stage definitions by name.
type Registry struct {
	defs map[string]Definition
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{defs: make(map[string]Definition)}
}

// Default is the registry used by the CLI. Builtins and custom steps add
// themselves to it in init, keeping the extension point in one place.
var Default = NewRegistry()

// Register adds a stage, rejecting empty or malformed names, nil
// definitions and duplicates instead of silently overwriting a step.
func (r *Registry) Register(name string, def Definition) error {
	switch {
	case name == "":
		return fmt.Errorf("step name must not be empty")
	case strings.ContainsAny(name, ": \t\n"):
		return fmt.Errorf("invalid step name %q", name)
	case def == nil:
		return fmt.Errorf("step %q has a nil definition", name)
	}
	if _, exists := r.defs[name]; exists {
		return fmt.Errorf("step %q already registered", name)
	}
	r.defs[name] = def
	return nil
}

// Must panics when Register fails: a duplicate builtin is a programming
// error, so fail fast rather than start with a missing step.
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// Names returns the registered step names in sorted order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.defs))
	for name := range r.defs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Pipeline is a compiled chain of stages.
type Pipeline struct {
	names  []string
	stages []Stage
}

// Compile resolves specs such as "upper" or "replace:old:new" into a
// pipeline. The first colon separates the name from its argument, which is
// passed through untouched (and may therefore contain colons itself).
func (r *Registry) Compile(specs []string) (*Pipeline, error) {
	pipeline := &Pipeline{}
	for i, spec := range specs {
		if strings.TrimSpace(spec) == "" {
			return nil, fmt.Errorf("step %d: empty step", i+1)
		}
		name, arg := spec, ""
		if colon := strings.IndexByte(spec, ':'); colon >= 0 {
			name, arg = spec[:colon], spec[colon+1:]
		}
		name = strings.TrimSpace(name)
		definition, ok := r.defs[name]
		if !ok {
			return nil, fmt.Errorf("step %d: unknown step %q", i+1, name)
		}
		stage, err := definition(arg)
		if err != nil {
			return nil, fmt.Errorf("step %d: %s: %w", i+1, name, err)
		}
		if stage == nil {
			return nil, fmt.Errorf("step %d: %s: nil stage", i+1, name)
		}
		pipeline.names = append(pipeline.names, name)
		pipeline.stages = append(pipeline.stages, stage)
	}
	return pipeline, nil
}

// Run applies the pipeline to every line of input and preserves whether the
// input ended with a newline.
func (p *Pipeline) Run(input string) (string, error) {
	if input == "" {
		return "", nil
	}
	endWithNewline := strings.HasSuffix(input, "\n")
	lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")
	for i, stage := range p.stages {
		next, err := stage(lines)
		if err != nil {
			return "", fmt.Errorf("step %d: %s: %w", i+1, p.names[i], err)
		}
		lines = next
		if len(lines) == 0 {
			return "", nil
		}
	}
	out := strings.Join(lines, "\n")
	if endWithNewline {
		out += "\n"
	}
	return out, nil
}
