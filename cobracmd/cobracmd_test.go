package cobracmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/strongo/buildinfo"
)

func newTestInfo() buildinfo.Info {
	return buildinfo.Info{
		Name:       "testcli",
		Version:    "1.2.3",
		Commit:     "abcdef1234567890",
		Date:       "2026-08-30T00:00:00Z",
		DateSource: buildinfo.DateSourceBuild,
	}
}

// runCobra drives plain cobra's own --version handling (root.Version and
// the version template set by WireCobra), with args as if typed on a
// command line. No fang involved - this is the exact entry point a
// fang-free consumer's main() calls.
func runCobra(t *testing.T, info buildinfo.Info, args []string) string {
	t.Helper()

	root := &cobra.Command{Use: "testcli"}
	WireCobra(root, info)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		t.Fatalf("root.Execute(%v) error = %v", args, err)
	}
	return buf.String()
}

func TestWireCobra_FlagAndSubcommandReportSameVersion(t *testing.T) {
	info := newTestInfo()

	flagOut := strings.TrimSpace(runCobra(t, info, []string{"--version"}))
	subOut := strings.TrimSpace(runCobra(t, info, []string{"version"}))

	if flagOut != info.Short() {
		t.Errorf("--version output = %q, want exactly Short() = %q", flagOut, info.Short())
	}
	if subOut != info.Long() {
		t.Errorf("`version` subcommand output = %q, want exactly Long() = %q", subOut, info.Long())
	}

	// The core assertion WireCobra exists to guarantee: the --version flag
	// (plain cobra, driven by root.Version) and the `version` subcommand
	// (our own RunE) must agree, because both were wired from the
	// identical Info by the same WireCobra call. Long()'s shape is
	// "<name> <version> (<commit>) <date>", so the version field is its
	// second whitespace-separated token.
	subFields := strings.Fields(subOut)
	if len(subFields) < 2 || subFields[1] != flagOut {
		t.Errorf("`version` subcommand output %q does not embed the same version reported by --version (%q)", subOut, flagOut)
	}
}

func TestWireCobra_ShorthandVersionFlag(t *testing.T) {
	info := newTestInfo()

	out := strings.TrimSpace(runCobra(t, info, []string{"-v"}))

	if out != info.Short() {
		t.Errorf("-v output = %q, want %q", out, info.Short())
	}
}

func TestWireCobra_VersionFlagHasNoDecoration(t *testing.T) {
	// WireCobra must set root.Version to exactly info.Short() and install
	// VersionTemplate, so plain cobra's built-in --version handling never
	// carries cobra's default "<name> version " prefix through to output.
	info := newTestInfo()

	out := strings.TrimSpace(runCobra(t, info, []string{"--version"}))

	if strings.Contains(out, "testcli version") {
		t.Errorf("--version output carried cobra's default prefix, got %q", out)
	}
	if out != info.Short() {
		t.Errorf("--version output = %q, want exactly %q", out, info.Short())
	}
}

func TestVersionCommand_PrintsLong(t *testing.T) {
	info := newTestInfo()
	cmd := VersionCommand(info)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != info.Long() {
		t.Errorf("VersionCommand output = %q, want %q", got, info.Long())
	}
}

func TestVersionCommand_JSON_PrintsExactlyOneContractObject(t *testing.T) {
	info := newTestInfo()
	cmd := VersionCommand(info)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := buf.String()
	if n := strings.Count(out, "\n"); n != 1 {
		t.Fatalf("output has %d newlines, want exactly 1 (one object, one trailing newline): %q", n, out)
	}

	var got buildinfo.VersionJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", out, err)
	}
	if got != info.JSON() {
		t.Errorf("decoded VersionJSON = %+v, want %+v", got, info.JSON())
	}
}

func TestVersionCommand_JSON_ViaWireCobraSubcommand(t *testing.T) {
	// Exercises the exact entry point WireCobra sets up, not just a
	// standalone VersionCommand(), so a future change to WireCobra's own
	// flag handling can't silently break `<cli> version --json`.
	info := newTestInfo()

	out := runCobra(t, info, []string{"version", "--json"})

	var got buildinfo.VersionJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", out, err)
	}
	if got != info.JSON() {
		t.Errorf("decoded VersionJSON = %+v, want %+v", got, info.JSON())
	}
}

func TestVersionCommand_PlainOutputUnaffectedByJSONFlagExisting(t *testing.T) {
	// Regression guard: adding the --json flag must not change plain
	// `version` output (byte-identical requirement).
	info := newTestInfo()

	out := runCobra(t, info, []string{"version"})

	if strings.TrimSpace(out) != info.Long() {
		t.Errorf("plain `version` output = %q, want %q", strings.TrimSpace(out), info.Long())
	}
}
