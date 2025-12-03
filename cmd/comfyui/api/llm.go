package api

import (
	"fmt"
)

// jsonschema is parsed by https://github.com/invopop/jsonschema

// 1. The Target Struct: This is what we want the data to look like in Go
type CreativeLists struct {
	// action examples: chasing bubbles, reading a giant book, riding a tiger
	Actions []string `json:"actions" jsonschema:"description=A list of creative activities or poses."`
	// context example: Ghibli style meadow
	Contexts []string `json:"contexts" jsonschema:"description=A list of detailed visual environments and art styles."`
}

func PromptActionList(subject string) string {
	return fmt.Sprintf(`I am generating AI images and videos of a subject of %s.
I need two lists to combine later:
1. 'actions': A list of 20 distinct, imaginative actions suitable for the subject (e.g., chasing bubbles, reading a giant book, riding a tiger).
2. 'contexts': A list of 20 distinct visual styles/environments (e.g., Ghibli style meadow, Cyberpunk street, Watercolor dream).

Be creative. Ensure variety in lighting, atmosphere, and activity.`, subject)
}
