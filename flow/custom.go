package main

import "strings"

// custom.go is the extension template: a stage registers itself from init
// and is immediately available to every pipeline, with no engine changes.
// Copy this pattern to add your own stage.
func init() {
	Must(Default.Register("rot13", Simple(rot13Encode)))
}

func rot13Encode(text string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+13)%26
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+13)%26
		}
		return r
	}, text)
}
