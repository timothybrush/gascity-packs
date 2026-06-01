package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeUserAliasFile writes a handle->Slack-ID JSON map to a tmpfile and
// returns its path. Tests use this to stage operator-edited
// slack-user-aliases.json states without exercising a write API (none
// exists in v1).
func writeUserAliasFile(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "slack-user-aliases.json")
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// newUserAliasMapForTest builds a userAliasMap from an in-memory binding
// set, failing the test on any load/parse error. Keeps rewrite-focused
// tests from repeating the write+construct dance.
func newUserAliasMapForTest(t *testing.T, entries map[string]string) *userAliasMap {
	t.Helper()
	path := writeUserAliasFile(t, t.TempDir(), entries)
	m, err := newUserAliasMap(path)
	if err != nil {
		t.Fatalf("newUserAliasMap: %v", err)
	}
	return m
}

// TestUserAliasRewrite is the core acceptance table: mapped user ->
// <@U…>, mapped group -> <!subteam^S…>, unmapped handle stays literal,
// leading vs mid-text both rewrite, and empty / no-token bodies pass
// through untouched. This is the defect gpk-uha7 fixes — an outbound
// "@mayor" must become a clickable, notifying mention, not literal text.
func TestUserAliasRewrite(t *testing.T) {
	m := newUserAliasMapForTest(t, map[string]string{
		"mayor":       "U0123ABCD",   // user
		"design-team": "S0456WXYZ",   // user group
		"cos":         "W0789ENTUSR", // Enterprise Grid org user
	})

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"mapped user leading", "@mayor please review", "<@U0123ABCD> please review"},
		{"mapped user mid-text", "ping @mayor now", "ping <@U0123ABCD> now"},
		{"mapped group", "heads up @design-team", "heads up <!subteam^S0456WXYZ>"},
		{"mapped W-prefixed user", "@cos fyi", "<@W0789ENTUSR> fyi"},
		{"unmapped stays literal", "@nobody hello", "@nobody hello"},
		{"mix mapped and unmapped", "@mayor and @nobody", "<@U0123ABCD> and @nobody"},
		{"multiple mapped tokens", "@mayor @design-team", "<@U0123ABCD> <!subteam^S0456WXYZ>"},
		{"empty body", "", ""},
		{"no token", "just plain text", "just plain text"},
		{"bare at with no handle", "email me @ noon", "email me @ noon"},
		{"handle with hyphen", "cc @design-team done", "cc <!subteam^S0456WXYZ> done"},
		{"token in parentheses", "(@mayor)", "(<@U0123ABCD>)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.rewrite(tc.in); got != tc.want {
				t.Errorf("rewrite(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestUserAliasRewriteEmailBoundary asserts the left-word-boundary guard:
// an `@handle` whose `@` is preceded by a handle character (the local
// part of an email-like "user@host") is NOT rewritten, even when the
// handle is mapped. Rewriting it would corrupt the address and could ping
// an unrelated target — the exact "surprise mention" failure the
// fail-safe design avoids.
func TestUserAliasRewriteEmailBoundary(t *testing.T) {
	m := newUserAliasMapForTest(t, map[string]string{
		"mayor": "U0123ABCD",
	})
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"email local part", "ping deploy@mayor for help", "ping deploy@mayor for help"},
		{"email full", "alerts@mayor.example.com", "alerts@mayor.example.com"},
		{"leading still rewrites", "@mayor", "<@U0123ABCD>"},
		{"after newline rewrites", "line one\n@mayor", "line one\n<@U0123ABCD>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.rewrite(tc.in); got != tc.want {
				t.Errorf("rewrite(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestUserAliasRewriteNilAndEmpty documents the nil-safety contract
// handlePublish relies on: a nil *userAliasMap (install never curated the
// file path) and an empty map both leave text untouched. Without this the
// publish path would need a nil-check at the call site.
func TestUserAliasRewriteNilAndEmpty(t *testing.T) {
	var nilMap *userAliasMap
	if got := nilMap.rewrite("@mayor hi"); got != "@mayor hi" {
		t.Errorf("nil map rewrite = %q, want unchanged", got)
	}
	if n := nilMap.Len(); n != 0 {
		t.Errorf("nil Len = %d, want 0", n)
	}
	if got := nilMap.All(); got != nil {
		t.Errorf("nil All = %v, want nil", got)
	}

	empty := newUserAliasMapForTest(t, map[string]string{})
	if got := empty.rewrite("@mayor hi"); got != "@mayor hi" {
		t.Errorf("empty map rewrite = %q, want unchanged", got)
	}
}

// TestUserAliasMapEmptyAndAbsent confirms the constructor accepts both
// "file does not exist" (locked-down workspace that never wrote the map)
// and "file is `{}`" (operator cleared all bindings) without crashing
// startup — a crash would block adapter launch on installs that haven't
// adopted outbound mention rewriting.
func TestUserAliasMapEmptyAndAbsent(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "slack-user-aliases.json")
	m, err := newUserAliasMap(missing)
	if err != nil {
		t.Fatalf("missing-file: %v", err)
	}
	if m.Len() != 0 {
		t.Errorf("missing-file Len = %d, want 0", m.Len())
	}

	empty := writeUserAliasFile(t, dir, map[string]string{})
	m2, err := newUserAliasMap(empty)
	if err != nil {
		t.Fatalf("empty-file: %v", err)
	}
	if m2.Len() != 0 {
		t.Errorf("empty-file Len = %d, want 0", m2.Len())
	}
}

// TestUserAliasMapRejectsCorruptEntries asserts parse fails loudly on
// mis-shaped input rather than silently dropping or mis-emitting a
// mention. A handle mapped to a non-user/non-group ID would otherwise
// produce a broken mention on the wire; a key carrying the `@` prefix or
// whitespace could never match the body scanner and would sit as dead
// config.
func TestUserAliasMapRejectsCorruptEntries(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		payload string
	}{
		{"empty key", `{"":"U0123"}`},
		{"channel id rejected", `{"mayor":"C0123ABCD"}`},
		{"bot id rejected", `{"mayor":"B0123ABCD"}`},
		{"lowercase id", `{"mayor":"u0123abcd"}`},
		{"id too short", `{"mayor":"U"}`},
		{"empty target", `{"mayor":""}`},
		{"handle with at prefix", `{"@mayor":"U0123"}`},
		{"handle with space", `{"the mayor":"U0123"}`},
		{"wrong value shape", `{"mayor":{"id":"U0123"}}`},
		{"not an object", `["mayor","U0123"]`},
		{"malformed json", `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, "bad-"+tc.name+".json")
			if err := os.WriteFile(path, []byte(tc.payload), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			if _, err := newUserAliasMap(path); err == nil {
				t.Errorf("payload %q parsed cleanly; want error", tc.payload)
			}
		})
	}
}

// TestUserAliasMapReloadOnUpdate confirms the SIGHUP reload path picks up
// operator edits without an adapter restart — the promise the startup log
// line makes ("read-only; SIGHUP or restart to reload"). The rewrite
// output must reflect the edited map after Reload.
func TestUserAliasMapReloadOnUpdate(t *testing.T) {
	dir := t.TempDir()
	path := writeUserAliasFile(t, dir, map[string]string{
		"mayor": "U0123ABCD",
	})
	m, err := newUserAliasMap(path)
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if got := m.rewrite("@mayor"); got != "<@U0123ABCD>" {
		t.Fatalf("initial rewrite = %q, want <@U0123ABCD>", got)
	}

	// Operator edits the file: repoint mayor, add a group binding.
	writeUserAliasFile(t, dir, map[string]string{
		"mayor": "U9999NEWID",
		"team":  "S0456WXYZ",
	})
	if err := m.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := m.rewrite("@mayor cc @team"); got != "<@U9999NEWID> cc <!subteam^S0456WXYZ>" {
		t.Errorf("after reload rewrite = %q, want <@U9999NEWID> cc <!subteam^S0456WXYZ>", got)
	}
	if m.Len() != 2 {
		t.Errorf("after reload Len = %d, want 2", m.Len())
	}
}

// TestUserAliasMapReloadRejectsCorruptPreservesLiveState asserts a failed
// reload (operator pushes a bad edit) leaves the previously-loaded
// bindings intact rather than blanking the map — corrupt input must not
// silently disable outbound rewriting.
func TestUserAliasMapReloadRejectsCorruptPreservesLiveState(t *testing.T) {
	dir := t.TempDir()
	path := writeUserAliasFile(t, dir, map[string]string{"mayor": "U0123ABCD"})
	m, err := newUserAliasMap(path)
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"mayor":"C0BADID"}`), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	if err := m.Reload(); err == nil {
		t.Fatalf("reload accepted corrupt edit; want error")
	}
	if got := m.rewrite("@mayor"); got != "<@U0123ABCD>" {
		t.Errorf("after failed reload rewrite = %q, want live state <@U0123ABCD>", got)
	}
}

// TestUserAliasMapAllSortedForDiffStability asserts All() returns entries
// in deterministic handle order so test fixtures and any operator-facing
// list output stay diff-stable across restarts.
func TestUserAliasMapAllSortedForDiffStability(t *testing.T) {
	m := newUserAliasMapForTest(t, map[string]string{
		"cos":   "UB000000",
		"mayor": "UA000000",
		"team":  "SC000000",
	})
	got := m.All()
	want := []string{"cos=<@UB000000>", "mayor=<@UA000000>", "team=<!subteam^SC000000>"}
	if !sort.StringsAreSorted(got) {
		t.Errorf("All() not sorted: %v", got)
	}
	if len(got) != len(want) {
		t.Fatalf("All() len = %d, want %d (got=%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("All()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestSlackMentionFor unit-tests the ID-prefix -> mention-syntax mapping
// in isolation: U/W -> user mention, S -> subteam mention, everything
// else rejected.
func TestSlackMentionFor(t *testing.T) {
	cases := []struct {
		id     string
		want   string
		wantOK bool
	}{
		{"U0123ABCD", "<@U0123ABCD>", true},
		{"W0123ABCD", "<@W0123ABCD>", true},
		{"S0123ABCD", "<!subteam^S0123ABCD>", true},
		{"C0123ABCD", "", false},  // channel
		{"B0123ABCD", "", false},  // bot
		{"u0123abcd", "", false},  // lowercase
		{"U", "", false},          // too short
		{"", "", false},           // empty
		{"U0123 ABCD", "", false}, // embedded space
	}
	for _, tc := range cases {
		got, ok := slackMentionFor(tc.id)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("slackMentionFor(%q) = (%q, %v), want (%q, %v)", tc.id, got, ok, tc.want, tc.wantOK)
		}
	}
}
