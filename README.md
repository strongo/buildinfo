# buildinfo

`github.com/strongo/buildinfo` is one shared way for Go CLIs in this fleet
to report their own version, commit and build date - instead of every CLI
hand-rolling its own `main.version` plumbing, disagreeing with whatever its
CLI framework does for `--version`, and eventually shipping something like:

```
ingitdb --version   →  ingitdb version unknown (built from source)
ingitdb version     →  ingitdb 0.65.11 (none) @ unknown
```

Two different mechanisms, two different (wrong) answers. This module fixes
that by giving every CLI **one identical `-ldflags -X` snippet** that
targets this module's package path, and (for the fleet's cobra +
[fang](https://github.com/charmbracelet/fang) stack) one `Wire` call that
feeds both the `--version`/`-v` flag and the `version` subcommand from the
exact same resolved value.

## Install

```
go get github.com/strongo/buildinfo
```

## Core API

```go
package buildinfo

type Info struct {
    Name    string // program name, e.g. "ingitdb"
    Version string // bare semver, no leading "v"
    Commit  string // full git SHA (with a "+dirty" suffix if the tree was
                    // dirty at build time), or "" when genuinely unknown
    Date    string // RFC 3339 / ISO 8601, or "" when genuinely unknown
}

// Get resolves name's build identity: link-time -X values first, falling
// back to runtime/debug.ReadBuildInfo(), falling back to clearly-marked
// placeholders. Never panics; safe to call before any linker stamping
// (e.g. in tests or `go run`).
func Get(name string) Info

// Short is exactly the bare version - for `--version`.
func (i Info) Short() string

// Long is "<name> <version> (<commit>) <date>" - for a `version` subcommand.
func (i Info) Long() string
```

Call `buildinfo.Get(name)` once at startup and use the returned `Info`
everywhere - never read the package's unexported `version`/`commit`/`date`
variables directly.

## The canonical ldflags snippet

Every CLI in the fleet should use this exact
[GoReleaser](https://goreleaser.com) `ldflags` block, verified against
GoReleaser's current template variable names
(`{{.Version}}`, `{{.FullCommit}}`, `{{.Date}}` - see
[GoReleaser's template docs](https://goreleaser.com/customization/templates/)):

```yaml
builds:
  - ldflags:
      - -s -w
      - -X github.com/strongo/buildinfo.version={{.Version}}
      - -X github.com/strongo/buildinfo.commit={{.FullCommit}}
      - -X github.com/strongo/buildinfo.date={{.Date}}
```

`{{.Version}}` is already stripped of its leading `v` by GoReleaser itself;
`Info.Version` strips it defensively too, so the `Short()`/`Long()` contract
holds even when `Get`'s `runtime/debug.ReadBuildInfo()` fallback path is hit
instead (module versions there keep the `v` prefix).

`-ldflags -X` can target a variable in any imported package, not just
`main` - that is the entire point of this module: point every CLI's
GoReleaser config at `github.com/strongo/buildinfo.{version,commit,date}`
and every binary reports through the identical, tested code path instead of
each CLI reinventing its own.

## Wiring: cobra + fang

The fleet standardises on [cobra](https://github.com/spf13/cobra) fronted by
[fang](https://github.com/charmbracelet/fang). Left on its own, fang
resolves `--version`/`-v` from `runtime/debug.ReadBuildInfo()` and prints
`"unknown (built from source)"` whenever that lookup comes up empty - which
is exactly the shipped bug above. The `cobracmd` subpackage closes that gap
by feeding fang and a `version` subcommand from the same `Info`:

```go
package main

import (
    "context"
    "os"

    "charm.land/fang/v2"
    "github.com/spf13/cobra"
    "github.com/strongo/buildinfo"
    "github.com/strongo/buildinfo/cobracmd"
)

func main() {
    info := buildinfo.Get("mycli")

    root := &cobra.Command{
        Use:   "mycli",
        Short: "mycli does things",
    }
    // ... attach the rest of the command tree to root ...

    fangOpts := cobracmd.Wire(root, info)
    if err := fang.Execute(context.Background(), root, fangOpts...); err != nil {
        os.Exit(1)
    }
}
```

`cobracmd.Wire`:

1. Adds a `version` subcommand to `root` that prints `info.Long()`.
2. Overrides cobra's default version template so the `--version`/`-v` flag
   prints exactly `info.Short()` - no `"<name> version "` prefix, no
   `fang`-appended commit suffix.
3. Returns the `fang.Option`s that make fang use that same `info.Short()`
   instead of its own `debug.ReadBuildInfo()` guess.

Because both surfaces read from the one `Info` value passed into `Wire`,
they cannot independently drift the way they did on the shipped binary
above - this is covered by a test (`cobracmd.TestWire_FlagAndSubcommandReportSameVersion`)
that drives real `fang.Execute` calls for both `--version` and `version`
and asserts they report the same version.

`cobracmd` is a separate package/import from the root `buildinfo` package,
so anything that only needs `Info`/`Get`/`Short`/`Long` never pulls in
cobra or fang.

## Output contract

- **`--version` / `-v` flag** (via `cobracmd.Wire` + `fang.Execute`, or any
  direct use of `Info.Short()`): exactly the bare semver version, nothing
  else. No program name, no commit, no date, no parentheses. Consumable as
  `$(mycli --version)`.

  ```
  $ mycli --version
  1.2.3
  ```

- **`version` subcommand** (via `cobracmd.VersionCommand`, or any direct
  use of `Info.Long()`): `"<name> <version> (<commit>) <date>"`. `commit`
  and `date` degrade to the literal string `unknown` when genuinely
  unresolved, so the shape never collapses to empty parens or a trailing
  gap.

  ```
  $ mycli version
  mycli 1.2.3 (a1b2c3d4e5f6...) 2026-08-30T12:00:00Z
  ```

Both outputs end with a single trailing newline supplied by the printing
caller (`Short()`/`Long()` themselves return no newline).

## Known traps this module already handles

- `runtime/debug.ReadBuildInfo()`'s `Deps` come back empty inside a
  non-main package's own test binary - `Get`'s fallback only reads
  `Main.Version` and `Settings` (`vcs.revision`, `vcs.time`,
  `vcs.modified`), never `Deps`, so this doesn't affect it.
- `go build` omits `vcs.revision` (and the rest of the `vcs.*` settings)
  when building from a linked git worktree rather than a normal checkout.
  `Get` treats a missing `vcs.revision` as "genuinely unknown" and degrades
  to the documented placeholders rather than panicking or fabricating a
  value.
- A dirty working tree (`vcs.modified == "true"`) is represented honestly
  as a `+dirty` suffix appended to `Commit` - but `Short()` only ever
  returns `Version`, so a dirty tree can never leak into `--version` output.
