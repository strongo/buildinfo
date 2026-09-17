package buildinfo

import (
	"encoding/json"
	"runtime/debug"
	"strings"
	"testing"
)

// resetStamps saves the package-level stamped vars, sets them to the given
// values for the duration of the test, and restores the originals on
// cleanup. Tests must not run in parallel with each other since these vars
// are shared package state (mirroring how the linker stamps them once for
// a whole process).
func resetStamps(t *testing.T, v, c, d string) {
	t.Helper()
	origV, origC, origD := version, commit, date
	version, commit, date = v, c, d
	t.Cleanup(func() {
		version, commit, date = origV, origC, origD
	})
}

func TestGet_StampedValuesWinOverBuildInfo(t *testing.T) {
	resetStamps(t, "1.2.3", "abcdef1234567890", "2026-08-30T00:00:00Z")

	got := Get("mycli")

	if got.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", got.Version, "1.2.3")
	}
	if got.Commit != "abcdef1234567890" {
		t.Errorf("Commit = %q, want %q", got.Commit, "abcdef1234567890")
	}
	if got.Date != "2026-08-30T00:00:00Z" {
		t.Errorf("Date = %q, want %q", got.Date, "2026-08-30T00:00:00Z")
	}
	if got.Name != "mycli" {
		t.Errorf("Name = %q, want %q", got.Name, "mycli")
	}
}

func TestGet_DateSourceIsBuildWhenStamped(t *testing.T) {
	resetStamps(t, "1.2.3", "abcdef1234567890", "2026-08-30T00:00:00Z")

	got := Get("mycli")

	if got.DateSource != DateSourceBuild {
		t.Errorf("DateSource = %q, want %q", got.DateSource, DateSourceBuild)
	}
}

func TestGet_VPrefixStripped(t *testing.T) {
	resetStamps(t, "v9.8.7", "deadbeef", "2026-08-30T00:00:00Z")

	got := Get("mycli")

	if got.Version != "9.8.7" {
		t.Errorf("Version = %q, want %q (leading v must be stripped)", got.Version, "9.8.7")
	}
}

func TestGet_MissingEverything_DegradesWithoutPanic(t *testing.T) {
	// Empty stamps force the runtime/debug.ReadBuildInfo() fallback path,
	// which in a `go test` binary is unreliable in exactly the ways the
	// brief calls out (Deps come back empty for a non-main package,
	// vcs.revision can be absent from a worktree build). We only assert
	// this never panics and never returns an empty Version.
	resetStamps(t, "", "", "")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Get panicked: %v", r)
		}
	}()

	got := Get("mycli")

	if got.Version == "" {
		t.Errorf("Version must never be empty, got %q", got.Version)
	}
	// Commit/Date are allowed to be "" (Long() covers rendering that).
}

func TestInfo_Short_ExactlyVersionNoDecoration(t *testing.T) {
	i := Info{Name: "mycli", Version: "1.2.3", Commit: "abcdef1", Date: "2026-08-30T00:00:00Z"}

	got := i.Short()

	if got != "1.2.3" {
		t.Errorf("Short() = %q, want exactly %q", got, "1.2.3")
	}
	if strings.Contains(got, "\n") {
		t.Errorf("Short() must not contain a newline, got %q", got)
	}
	if strings.Contains(got, "mycli") || strings.Contains(got, "abcdef1") {
		t.Errorf("Short() must contain only the version, got %q", got)
	}
}

func TestInfo_Short_NeverLeaksDirtySuffix(t *testing.T) {
	// Short() must report the bare version even when Commit carries a
	// +dirty marker - dirtiness is a commit-level fact, not a version fact.
	i := Info{Name: "mycli", Version: "1.2.3", Commit: "abcdef1+dirty", Date: "2026-08-30T00:00:00Z"}

	got := i.Short()

	if got != "1.2.3" {
		t.Errorf("Short() = %q, want %q (must not leak +dirty)", got, "1.2.3")
	}
	if strings.Contains(got, "dirty") {
		t.Errorf("Short() leaked dirty marker: %q", got)
	}
}

func TestInfo_Long_MatchesDocumentedShape(t *testing.T) {
	i := Info{Name: "mycli", Version: "1.2.3", Commit: "abcdef1234567890", Date: "2026-08-30T00:00:00Z"}

	want := "mycli 1.2.3 (abcdef1234567890) 2026-08-30T00:00:00Z"
	if got := i.Long(); got != want {
		t.Errorf("Long() = %q, want %q", got, want)
	}
}

func TestInfo_Long_MissingCommitAndDate_DegradeToPlaceholders(t *testing.T) {
	i := Info{Name: "mycli", Version: "1.2.3", Commit: "", Date: ""}

	want := "mycli 1.2.3 (unknown) unknown"
	if got := i.Long(); got != want {
		t.Errorf("Long() = %q, want %q", got, want)
	}
}

func TestInfo_Long_DirtyTreeMarking(t *testing.T) {
	i := Info{Name: "mycli", Version: "1.2.3", Commit: "abcdef1+dirty", Date: "2026-08-30T00:00:00Z"}

	want := "mycli 1.2.3 (abcdef1+dirty) 2026-08-30T00:00:00Z"
	if got := i.Long(); got != want {
		t.Errorf("Long() = %q, want %q", got, want)
	}
}

func TestApplyBuildInfo_DirtyTreeAppendsSuffix(t *testing.T) {
	var info Info
	bi := fakeBuildInfo("v0.1.0", "cafebabe", "2026-08-30T12:00:00Z", true)

	applyBuildInfo(&info, bi)

	if info.Commit != "cafebabe+dirty" {
		t.Errorf("Commit = %q, want %q", info.Commit, "cafebabe+dirty")
	}
}

func TestApplyBuildInfo_CleanTreeNoSuffix(t *testing.T) {
	var info Info
	bi := fakeBuildInfo("v0.1.0", "cafebabe", "2026-08-30T12:00:00Z", false)

	applyBuildInfo(&info, bi)

	if info.Commit != "cafebabe" {
		t.Errorf("Commit = %q, want %q", info.Commit, "cafebabe")
	}
}

func TestApplyBuildInfo_DevelVersionIgnored(t *testing.T) {
	var info Info
	bi := fakeBuildInfo("(devel)", "cafebabe", "2026-08-30T12:00:00Z", false)

	applyBuildInfo(&info, bi)

	if info.Version != "" {
		t.Errorf("Version = %q, want empty ((devel) must be ignored, not surfaced)", info.Version)
	}
}

func TestApplyBuildInfo_DoesNotOverwriteAlreadyStamped(t *testing.T) {
	info := Info{Version: "9.9.9", Commit: "already-set"}
	bi := fakeBuildInfo("v0.1.0", "cafebabe", "2026-08-30T12:00:00Z", false)

	applyBuildInfo(&info, bi)

	if info.Version != "9.9.9" {
		t.Errorf("Version was overwritten: %q", info.Version)
	}
	if info.Commit != "already-set" {
		t.Errorf("Commit was overwritten: %q", info.Commit)
	}
}

func TestApplyBuildInfo_DateSourceCommitWhenDateFilledFromVCSTime(t *testing.T) {
	var info Info
	bi := fakeBuildInfo("v0.1.0", "cafebabe", "2026-08-30T12:00:00Z", false)

	applyBuildInfo(&info, bi)

	if info.Date != "2026-08-30T12:00:00Z" {
		t.Errorf("Date = %q, want %q", info.Date, "2026-08-30T12:00:00Z")
	}
	if info.DateSource != DateSourceCommit {
		t.Errorf("DateSource = %q, want %q", info.DateSource, DateSourceCommit)
	}
}

func TestApplyBuildInfo_DoesNotOverwriteAlreadyStampedDateOrSource(t *testing.T) {
	info := Info{Date: "2026-01-01T00:00:00Z", DateSource: DateSourceBuild}
	bi := fakeBuildInfo("v0.1.0", "cafebabe", "2026-08-30T12:00:00Z", false)

	applyBuildInfo(&info, bi)

	if info.Date != "2026-01-01T00:00:00Z" {
		t.Errorf("Date was overwritten: %q", info.Date)
	}
	if info.DateSource != DateSourceBuild {
		t.Errorf("DateSource was overwritten: %q, want %q", info.DateSource, DateSourceBuild)
	}
}

func TestApplyBuildInfo_NoVCSTime_DateSourceStaysEmpty(t *testing.T) {
	var info Info
	bi := &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}

	applyBuildInfo(&info, bi)

	if info.Date != "" {
		t.Errorf("Date = %q, want empty", info.Date)
	}
	if info.DateSource != "" {
		t.Errorf("DateSource = %q, want empty", info.DateSource)
	}
}

func TestInfo_JSON_MapsAllFields(t *testing.T) {
	i := Info{
		Name:       "mycli",
		Version:    "1.2.3",
		Commit:     "abcdef1234567890",
		Date:       "2026-08-30T00:00:00Z",
		DateSource: DateSourceBuild,
	}

	want := VersionJSON{
		Name:       "mycli",
		Version:    "1.2.3",
		Commit:     "abcdef1234567890",
		Date:       "2026-08-30T00:00:00Z",
		DateSource: DateSourceBuild,
	}
	if got := i.JSON(); got != want {
		t.Errorf("JSON() = %+v, want %+v", got, want)
	}
}

func TestInfo_JSON_ZeroValueHasEmptyDateSource(t *testing.T) {
	got := Info{Name: "mycli", Version: "dev"}.JSON()

	if got.DateSource != "" {
		t.Errorf("DateSource = %q, want empty for an unresolved date", got.DateSource)
	}
	if got.Date != "" {
		t.Errorf("Date = %q, want empty", got.Date)
	}
}

func TestVersionJSON_MarshalsContractKeys(t *testing.T) {
	v := VersionJSON{
		Name:       "mycli",
		Version:    "1.2.3",
		Commit:     "abcdef1234567890",
		Date:       "2026-08-30T00:00:00Z",
		DateSource: DateSourceBuild,
	}

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	want := map[string]string{
		"name":        "mycli",
		"version":     "1.2.3",
		"commit":      "abcdef1234567890",
		"date":        "2026-08-30T00:00:00Z",
		"date_source": "build",
	}
	if len(m) != len(want) {
		t.Fatalf("marshaled object has %d keys, want %d: %v", len(m), len(want), m)
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("key %q = %q, want %q", k, m[k], v)
		}
	}
}
