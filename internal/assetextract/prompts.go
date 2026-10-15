package assetextract

import (
	"embed"
	"strings"
)

//go:embed prompts/*.md
var assetPrompts embed.FS

// Each extraction type owns its system policy. Image APIs receive it in the
// prompt because their edit protocol does not expose a system-message field.
func assetSystemPrompt(kind string) string {
	data, err := assetPrompts.ReadFile("prompts/" + kind + ".md")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
