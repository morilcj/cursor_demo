package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
)

func main() {
	root := flag.String("path", ".", "directory to scan")
	norecurse := flag.Bool("norecurse", false, "only immediate children of path (no subdirectories)")
	all := flag.Bool("all", false, "do not skip common directories (.git, vendor, node_modules, …)")
	extraSkip := flag.String("skip", "", "comma-separated extra directory basenames to skip (always applied)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Summarize file counts and newline totals grouped by extension (like wc -l for text files).\n")
		fmt.Fprintf(os.Stderr, "Binary files are detected by a NUL byte in the first %d bytes; they do not contribute to line totals.\n\n", sniffLen)
		flag.PrintDefaults()
	}

	flag.Parse()

	skip := map[string]struct{}{}
	if !*all {
		skip = mergeSkip(defaultSkipDirs, parseSkipExtra(*extraSkip))
	} else {
		skip = parseSkipExtra(*extraSkip)
	}

	res := scanTree(*root, *norecurse, skip)

	for _, err := range res.errs {
		fmt.Fprintf(os.Stderr, "warn: %v\n", err)
	}

	fmt.Printf("Root:          %s\n", res.root)
	fmt.Printf("Total files:   %d\n", res.totalFiles)
	fmt.Printf("Total lines:   %d  (text files, newline count)\n", res.totalLines)
	if res.binaryFiles > 0 {
		fmt.Printf("Binary files:  %d  (excluded from line count)\n", res.binaryFiles)
	}
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "EXT\tFILES\tLINES\tBINARY")
	for _, row := range sortedRows(res.byExt) {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\n", row.ext, row.files, row.lines, row.binary)
	}
	w.Flush()
}
