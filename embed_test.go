package gascitypacks

import (
	"io/fs"
	"testing"
)

func TestGastownEmbedsPackContent(t *testing.T) {
	pack := Gastown()
	for _, rel := range []string{
		"pack.toml",
		"agents/dog/agent.toml",
		"agents/dog/prompt.template.md",
		"formulas/mol-shutdown-dance.toml",
		"template-fragments/propulsion.template.md",
		"overlay/per-provider/codex/.codex/hooks.json",
	} {
		if _, err := fs.Stat(pack, rel); err != nil {
			t.Errorf("gastown pack missing %s: %v", rel, err)
		}
	}
}

func TestGastownEmbedHasNoUnexpectedRoots(t *testing.T) {
	entries, err := fs.ReadDir(packsFS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "gastown" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("embedded roots = %v, want [gastown]", names)
	}
}
