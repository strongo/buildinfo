package fangcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"charm.land/fang/v2"
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

// runFang drives the exact fang.Execute entry point a real CLI's main()
// calls, with args as if typed on a command line. It genuinely exercises
// fang's own --version handling (fang.buildVersion, cobra's version flag
// and template machinery) rather than calling internal helpers directly.
func runFang(t *testing.T, info buildinfo.Info, args []string) string {
	t.Helper()

	root := &cobra.Command{Use: "testcli"}
	opts := Wire(root, info)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)

	if err := fang.Execute(context.Background(), root, opts...); err != nil {
		t.Fatalf("fang.Execute(%v) error = %v", args, err)
	}
	return buf.String()
}

func TestWire_FlagAndSubcommandReportSameVersion(t *testing.T) {
	info := newTestInfo()

	flagOut := strings.TrimSpace(runFang(t, info, []string{"--version"}))
	subOut := strings.TrimSpace(runFang(t, info, []string{"version"}))

	if flagOut != info.Short() {
		t.Errorf("--version output = %q, want exactly Short() = %q", flagOut, info.Short())
	}
	if subOut != info.Long() {
		t.Errorf("`version` subcommand output = %q, want exactly Long() = %q", subOut, info.Long())
	}

	// The core assertion this module exists to guarantee: the --version
	// flag (resolved by fang) and the `version` subcommand (our own
	// RunE) must agree on the version, because both were wired from the
	// identical Info by the same Wire call - they cannot independently
	// drift the way fang's build-info lookup and a hand-rolled subcommand
	// did on the shipped ingitdb-cli binary this module exists to fix.
	// Long()'s shape is "<name> <version> (<commit>) <date>", so the
	// version field is its second whitespace-separated token.
	subFields := strings.Fields(subOut)
	if len(subFields) < 2 || subFields[1] != flagOut {
		t.Errorf("`version` subcommand output %q does not embed the same version reported by --version (%q)", subOut, flagOut)
	}
}

func TestWire_ShorthandVersionFlag(t *testing.T) {
	info := newTestInfo()

	out := strings.TrimSpace(runFang(t, info, []string{"-v"}))

	if out != info.Short() {
		t.Errorf("-v output = %q, want %q", out, info.Short())
	}
}

func TestWire_VersionFlagHasNoDecoration(t *testing.T) {
	// Regression guard for the exact shipped bug this module fixes: fang's
	// default --version resolution printed "unknown (built from source)"
	// because it fell through to its own debug.ReadBuildInfo() lookup with
	// nothing stamped. Wire must feed fang a real, undecorated version so
	// that never happens, and the output must never contain fang's
	// parenthesized commit suffix (Short()'s contract has no commit at all).
	info := newTestInfo()

	out := strings.TrimSpace(runFang(t, info, []string{"--version"}))

	if strings.Contains(out, "unknown") {
		t.Errorf("--version output regressed to fang's unresolved placeholder: %q", out)
	}
	if strings.Contains(out, "(") {
		t.Errorf("--version output must not carry a commit suffix, got %q", out)
	}
}

func TestWire_VersionJSON_PrintsExactlyOneContractObject(t *testing.T) {
	// fangcmd.Wire adds cobracmd.VersionCommand as-is, so `version --json`
	// must work identically when driven through fang.Execute - this proves
	// fang's own flag/command handling doesn't swallow or reinterpret the
	// --json flag on the version subcommand.
	info := newTestInfo()

	out := runFang(t, info, []string{"version", "--json"})

	var got buildinfo.VersionJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", out, err)
	}
	if got != info.JSON() {
		t.Errorf("decoded VersionJSON = %+v, want %+v", got, info.JSON())
	}
}
