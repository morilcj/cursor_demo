// Command flow runs each line of stdin through a configurable chain of
// stages. Run it with -list to see the registered stages.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	steps := flag.String("steps", "", "comma-separated stages, e.g. upper,replace:old:new")
	list := flag.Bool("list", false, "list available stages and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -steps STAGES\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Read lines from stdin, run every line through STAGES and print the result.")
		fmt.Fprintln(os.Stderr, "Stages are comma-separated. An argument follows a colon, e.g. prefix:> .")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  printf 'b\\na\\n' | go run ./flow -steps sort,number")
		fmt.Fprintln(os.Stderr, "  go run ./flow -list")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *list {
		for _, name := range Default.Names() {
			fmt.Println(name)
		}
		return
	}
	if strings.TrimSpace(*steps) == "" {
		fail("missing -steps; use -list to see available stages")
	}

	pipeline, err := Default.Compile(strings.Split(*steps, ","))
	if err != nil {
		fail(err.Error())
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail("read stdin: " + err.Error())
	}
	output, err := pipeline.Run(string(input))
	if err != nil {
		fail(err.Error())
	}
	fmt.Print(output)
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "flow: %s\n", message)
	os.Exit(1)
}
