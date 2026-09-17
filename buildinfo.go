// Package buildinfo provides one framework-agnostic way for Go CLIs in this
// fleet to report their own version, commit and build date.
//
// The variables below are meant to be set at link time with -ldflags -X,
// targeting this exact package path (github.com/strongo/buildinfo), so every
// CLI can share the identical ldflags snippet instead of hand-rolling its
// own main.version plumbing. See the README for the canonical GoReleaser
// snippet.
//
// Callers should never read version, commit or date directly — call Get
// instead, which falls back to runtime/debug.ReadBuildInfo() when the
// linker did not stamp these variables (e.g. `go run`, `go test`, or a
// binary built without -ldflags).
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Stamped at link time via -ldflags "-X github.com/strongo/buildinfo.version=... \
// -X github.com/strongo/buildinfo.commit=... -X github.com/strongo/buildinfo.date=...".
// Do not read these directly; use Get.
var (
	version string
	commit  string
	date    string
)

// Date source labels for Info.DateSource / VersionJSON.DateSource, per the
// fleet-wide `version --json` contract (cli-install#req:version-json-contract
// in strongo/cli-helpers): callers must be able to tell a release build's
// release build time apart from a plain `go build`'s commit timestamp.
const (
	// DateSourceBuild marks a Date stamped at link time via -ldflags -X
	// (this package's date variable) - a released build's release build
	// time.
	DateSourceBuild = "build"
	// DateSourceCommit marks a Date read from runtime/debug.BuildInfo's
	// vcs.time - the commit's own timestamp, used when no link-time date
	// was stamped (e.g. a plain `go build` or `go install`).
	DateSourceCommit = "commit"
)

// Info is a resolved, ready-to-print snapshot of a program's build identity.
type Info struct {
	// Name is the program name, e.g. "ingitdb".
	Name string
	// Version is a bare semver string, with any leading "v" stripped.
	// It is "dev" when no version could be determined by any means.
	Version string
	// Commit is the full git SHA, with a "+dirty" suffix appended when the
	// build tree had uncommitted changes. It is "" when genuinely unknown.
	Commit string
	// Date is an RFC 3339 / ISO 8601 timestamp. It is "" when genuinely
	// unknown.
	Date string
	// DateSource records where Date came from: DateSourceBuild when it was
	// stamped at link time, DateSourceCommit when it was read from
	// runtime/debug.BuildInfo's vcs.time, or "" when Date itself is
	// unknown.
	DateSource string
}

// VersionJSON is the fleet-wide `version --json` contract
// (cli-install#req:version-json-contract in strongo/cli-helpers): the exact
// set of string keys every catalog CLI's `version --json` must print. The
// writer (Info.JSON(), used by cobracmd.VersionCommand and, through it,
// fangcmd.Wire) and any reader (e.g. cli-helpers' status prober) share this
// one exported type so the two can never decode a different shape from what
// was written. New keys may be added in the future; existing keys are never
// removed or repurposed.
type VersionJSON struct {
	// Name is the binary name, equal to its catalog id.
	Name string `json:"name"`
	// Version is the bare semver version, no leading "v"; "dev" when the
	// build cannot determine it.
	Version string `json:"version"`
	// Commit is the full commit SHA, with a "+dirty" suffix when built
	// from a modified tree; "" when unknown.
	Commit string `json:"commit"`
	// Date is an RFC 3339 timestamp; "" when unknown.
	Date string `json:"date"`
	// DateSource is DateSourceBuild, DateSourceCommit, or "" when Date
	// itself is unknown.
	DateSource string `json:"date_source"`
}

// JSON returns i as the fleet-wide version --json contract value. It never
// performs I/O; callers encode the result themselves (see
// cobracmd.VersionCommand for the canonical `version --json` writer).
//
// Info and VersionJSON share identical field names, order and types by
// design (only VersionJSON carries json tags), so this is a plain
// conversion rather than a field-by-field copy that could drift out of
// sync as either type grows.
func (i Info) JSON() VersionJSON {
	return VersionJSON(i)
}

// unknownVersion is the clearly-marked placeholder Get returns when no
// version could be resolved by any of its fallbacks.
const unknownVersion = "dev"

// Get resolves this program's build identity.
//
// Resolution order:
//  1. Link-time -X values (version, commit, date package vars), when
//     non-empty.
//  2. runtime/debug.ReadBuildInfo(): the main module's Version, plus
//     vcs.revision / vcs.time / vcs.modified from Settings, for whichever
//     of version/commit/date step 1 left unresolved.
//  3. Clearly-marked placeholders ("dev" for version, "" for commit/date).
//     Get never panics and never returns a misleading value.
//
// Get is safe to call before any linker stamping (e.g. in tests or `go run`).
func Get(name string) Info {
	info := Info{
		Name:    name,
		Version: version,
		Commit:  commit,
		Date:    date,
	}
	if info.Date != "" {
		info.DateSource = DateSourceBuild
	}

	if info.Version == "" || info.Commit == "" || info.Date == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			applyBuildInfo(&info, bi)
		}
	}

	info.Version = strings.TrimPrefix(info.Version, "v")
	if info.Version == "" {
		info.Version = unknownVersion
	}

	return info
}

// applyBuildInfo fills in whichever fields of info are still empty using
// bi, the result of debug.ReadBuildInfo(). It never overwrites a value
// already set from link-time stamping.
func applyBuildInfo(info *Info, bi *debug.BuildInfo) {
	if info.Version == "" {
		// (devel) is the placeholder Go itself uses for a main module built
		// without a resolvable version (e.g. from within its own repo
		// without a tag) - it carries no real information, so treat it the
		// same as "unset" rather than surfacing it verbatim.
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			info.Version = v
		}
	}

	var (
		revision string
		modified bool
	)
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			if info.Date == "" && s.Value != "" {
				info.Date = s.Value
				info.DateSource = DateSourceCommit
			}
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if info.Commit == "" && revision != "" {
		info.Commit = revision
		if modified {
			info.Commit += "+dirty"
		}
	}
}

// Short renders just the bare semver version, suitable for a `--version`
// flag consumed as `$(mycli --version)`: exactly the version string, no
// program name, no commit, no date, no decoration, and no trailing newline
// (the caller's print statement supplies that).
func (i Info) Short() string {
	return i.Version
}

// Long renders the full build identity, suitable for a `version` subcommand:
//
//	"<name> <version> (<commit>) <date>"
//
// Commit and date fall back to "unknown" when genuinely unresolved so the
// shape never collapses to empty parens or a trailing gap.
func (i Info) Long() string {
	commit := i.Commit
	if commit == "" {
		commit = "unknown"
	}
	date := i.Date
	if date == "" {
		date = "unknown"
	}
	return i.Name + " " + i.Version + " (" + commit + ") " + date
}
