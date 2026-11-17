package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Builtin stages register themselves in init, next to the helpers they use.
// Each one is a small, independent Definition, which is exactly the point:
// the pipeline engine knows nothing about any of them.
func init() {
	Must(Default.Register("upper", Simple(strings.ToUpper)))
	Must(Default.Register("lower", Simple(strings.ToLower)))
	Must(Default.Register("trim", Simple(strings.TrimSpace)))
	Must(Default.Register("reverse", Simple(reverseRunes)))
	Must(Default.Register("prefix", textStep("prefix", func(line, text string) string { return text + line })))
	Must(Default.Register("suffix", textStep("suffix", func(line, text string) string { return line + text })))
	Must(Default.Register("replace", replaceStep))
	Must(Default.Register("number", numberStep))
	Must(Default.Register("grep", filterStep("grep", true)))
	Must(Default.Register("drop", filterStep("drop", false)))
	Must(Default.Register("field", fieldStep))
	Must(Default.Register("sort", streamStep(sortLines)))
	Must(Default.Register("rsort", streamStep(reverseSortedLines)))
	Must(Default.Register("uniq", streamStep(uniqLines)))
	Must(Default.Register("head", limitStep(true)))
	Must(Default.Register("tail", limitStep(false)))
	Must(Default.Register("wrap", wrapStep))
	Must(Default.Register("join", joinStep))
}

func reverseRunes(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// textStep builds the shared shape of prefix and suffix.
func textStep(name string, combine func(line, text string) string) Definition {
	return func(arg string) (Stage, error) {
		if arg == "" {
			return nil, fmt.Errorf("want %s:TEXT", name)
		}
		return PerLine(func(line string) (string, error) {
			return combine(line, arg), nil
		}), nil
	}
}

func replaceStep(arg string) (Stage, error) {
	old, replacement, found := strings.Cut(arg, ":")
	if !found || old == "" {
		return nil, fmt.Errorf("want replace:OLD:NEW")
	}
	return PerLine(func(line string) (string, error) {
		return strings.ReplaceAll(line, old, replacement), nil
	}), nil
}

// numberStep is stateful: the counter lives in the Definition closure, so
// each compiled pipeline numbers its own stream from 1.
func numberStep(arg string) (Stage, error) {
	separator := arg
	if separator == "" {
		separator = ": "
	}
	lineNumber := 0
	return PerLine(func(line string) (string, error) {
		lineNumber++
		return strconv.Itoa(lineNumber) + separator + line, nil
	}), nil
}

func filterStep(name string, keep bool) Definition {
	return func(arg string) (Stage, error) {
		if arg == "" {
			return nil, fmt.Errorf("want %s:TEXT", name)
		}
		return func(lines []string) ([]string, error) {
			out := make([]string, 0, len(lines))
			for _, line := range lines {
				if strings.Contains(line, arg) == keep {
					out = append(out, line)
				}
			}
			return out, nil
		}, nil
	}
}

func fieldStep(arg string) (Stage, error) {
	number, separator, found := strings.Cut(arg, ":")
	if !found || separator == "" {
		return nil, fmt.Errorf("want field:N:SEPARATOR")
	}
	index, err := strconv.Atoi(strings.TrimSpace(number))
	if err != nil || index < 1 {
		return nil, fmt.Errorf("field index must be a positive integer")
	}
	return PerLine(func(line string) (string, error) {
		fields := strings.Split(line, separator)
		if index > len(fields) {
			return "", nil
		}
		return fields[index-1], nil
	}), nil
}

func streamStep(transform func([]string) []string) Definition {
	return func(arg string) (Stage, error) {
		if arg != "" {
			return nil, fmt.Errorf("takes no argument")
		}
		return func(lines []string) ([]string, error) {
			return transform(lines), nil
		}, nil
	}
}

func sortLines(lines []string) []string {
	out := append([]string(nil), lines...)
	sort.Strings(out)
	return out
}

func reverseSortedLines(lines []string) []string {
	out := sortLines(lines)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func uniqLines(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

func limitStep(head bool) Definition {
	name := "tail"
	if head {
		name = "head"
	}
	return func(arg string) (Stage, error) {
		n, err := parseCount(arg, name+":N")
		if err != nil {
			return nil, err
		}
		return func(lines []string) ([]string, error) {
			if n >= len(lines) {
				return lines, nil
			}
			if head {
				return lines[:n], nil
			}
			return lines[len(lines)-n:], nil
		}, nil
	}
}

func wrapStep(arg string) (Stage, error) {
	width, err := parseCount(arg, "wrap:N")
	if err != nil {
		return nil, err
	}
	if width == 0 {
		return nil, fmt.Errorf("wrap width must be positive")
	}
	return func(lines []string) ([]string, error) {
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			out = append(out, wrapLine(line, width)...)
		}
		return out, nil
	}, nil
}

// wrapLine breaks a line on spaces without splitting words. A word longer
// than width is emitted on its own, so no content is lost.
func wrapLine(line string, width int) []string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}
	out := make([]string, 0, len(words))
	current, currentWidth := words[0], len([]rune(words[0]))
	for _, word := range words[1:] {
		wordWidth := len([]rune(word))
		if currentWidth+1+wordWidth > width {
			out = append(out, current)
			current, currentWidth = word, wordWidth
			continue
		}
		current += " " + word
		currentWidth += 1 + wordWidth
	}
	return append(out, current)
}

func joinStep(arg string) (Stage, error) {
	return func(lines []string) ([]string, error) {
		return []string{strings.Join(lines, arg)}, nil
	}, nil
}

func parseCount(arg, usage string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("want %s with a non-negative integer", usage)
	}
	return n, nil
}
