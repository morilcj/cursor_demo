package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const sniffLen = 8192

var defaultSkipDirs = map[string]struct{}{
	".git":         {},
	".svn":         {},
	".hg":          {},
	"vendor":       {},
	"node_modules": {},
	"__pycache__":  {},
	".idea":        {},
}

type extAgg struct {
	files  int
	lines  int64
	binary int
}

type scanResult struct {
	root        string
	totalFiles  int
	totalLines  int64
	binaryFiles int
	byExt       map[string]*extAgg
	errs        []error
}

func shouldSkipDir(name string, skip map[string]struct{}) bool {
	_, ok := skip[name]
	return ok
}

func isBinary(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, sniffLen)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}
	return bytes.Contains(buf[:n], []byte{0}), nil
}

func countNewlines(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var n int64
	buf := make([]byte, 64*1024)
	for {
		br, err := f.Read(buf)
		for i := 0; i < br; i++ {
			if buf[i] == '\n' {
				n++
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func extKey(path string) string {
	base := filepath.Base(path)
	switch base {
	case "go.mod", "go.sum", "go.work", "go.work.sum",
		"Makefile", "Dockerfile", "LICENSE", "README", "README.md":
		return base
	}
	ext := filepath.Ext(path)
	if ext == "" {
		return "(none)"
	}
	return strings.ToLower(ext)
}

func scanTree(root string, norecurse bool, skip map[string]struct{}) *scanResult {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return &scanResult{
			root:  root,
			errs:  []error{err},
			byExt: make(map[string]*extAgg),
		}
	}

	res := &scanResult{
		root:  rootAbs,
		byExt: make(map[string]*extAgg),
	}

	walkErr := filepath.WalkDir(rootAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			res.errs = append(res.errs, err)
			return nil
		}

		if fi, statErr := os.Lstat(path); statErr == nil && fi.Mode()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			res.errs = append(res.errs, err)
			return nil
		}

		if norecurse && rel != "." {
			if strings.Contains(rel, string(filepath.Separator)) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Single path segment under root: skip descending into subdirectories.
			if d.IsDir() {
				return filepath.SkipDir
			}
		}

		if d.IsDir() {
			if rel == "." {
				return nil
			}
			if shouldSkipDir(d.Name(), skip) {
				return filepath.SkipDir
			}
			return nil
		}

		res.totalFiles++

		key := extKey(path)
		agg := res.byExt[key]
		if agg == nil {
			agg = &extAgg{}
			res.byExt[key] = agg
		}
		agg.files++

		bin, err := isBinary(path)
		if err != nil {
			res.errs = append(res.errs, err)
			return nil
		}
		if bin {
			agg.binary++
			res.binaryFiles++
			return nil
		}

		lines, err := countNewlines(path)
		if err != nil {
			res.errs = append(res.errs, err)
			return nil
		}
		agg.lines += lines
		res.totalLines += lines
		return nil
	})

	if walkErr != nil {
		res.errs = append(res.errs, walkErr)
	}

	return res
}

type extRow struct {
	ext    string
	files  int
	lines  int64
	binary int
}

func sortedRows(m map[string]*extAgg) []extRow {
	rows := make([]extRow, 0, len(m))
	for ext, a := range m {
		rows = append(rows, extRow{ext: ext, files: a.files, lines: a.lines, binary: a.binary})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].lines != rows[j].lines {
			return rows[i].lines > rows[j].lines
		}
		return rows[i].ext < rows[j].ext
	})
	return rows
}

func parseSkipExtra(s string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out[p] = struct{}{}
		}
	}
	return out
}

func mergeSkip(defaults, extra map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{})
	for k := range defaults {
		out[k] = struct{}{}
	}
	for k := range extra {
		out[k] = struct{}{}
	}
	return out
}
