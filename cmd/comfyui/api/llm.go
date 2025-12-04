package api

import (
	"fmt"
)

// jsonschema is parsed by https://github.com/invopop/jsonschema

// 1. The Target Struct: This is what we want the data to look like in Go
type CreativeLists struct {
	// action examples: chasing bubbles, reading a giant book, riding a tiger
	Actions   []string `json:"actions" jsonschema:"description=A list of creative activities or poses."`
	ActionsZh []string `json:"actions_zh" jsonschema:"description=The Simplified Chinese translation of actions"`
	// context example: Ghibli style meadow
	Contexts []string `json:"contexts" jsonschema:"description=A list of detailed visual environments and art styles."`
}

func PromptActionList(subject string) string {
	return fmt.Sprintf(`I am generating AI images and videos of a subject of %s.
I need two lists to combine later:
1. 'actions': A list of 50 distinct, imaginative actions suitable for the subject (e.g., chasing bubbles, reading a giant book, riding a tiger).
2. 'contexts': A list of 50 distinct visual styles/environments (e.g., Ghibli style meadow, Cyberpunk street, Watercolor dream).

Additionally, output a 'actions_zh' list that's the Simplified Chinese translation of 'actions' list. The 'actions' list and 'actions_zh' list must have the same length.

Be creative. Ensure variety in lighting, atmosphere, and activity.`, subject)
}
