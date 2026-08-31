// Package cobracmd wires github.com/strongo/buildinfo into a plain cobra
// command tree - no other CLI framework required. It gives a CLI's
// --version/-v flag and its `version` subcommand the same resolved
// buildinfo.Info, so the two surfaces cannot disagree.
//
// This package is fang-free by design and imports nothing beyond cobra
// and buildinfo itself: a binary that imports only cobracmd never links
// charm.land/fang/v2 or its terminal-UI dependencies. For the fleet's
// cobra + fang stack, use the sibling fangcmd package
// (github.com/strongo/buildinfo/fangcmd) instead - it wires the same
// Info through fang.Execute and reuses VersionCommand from this package
// so the two wiring paths cannot drift onto different output.
package cobracmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/strongo/buildinfo"
)

// VersionTemplate overrides cobra's default "<name> version <version>\n"
// so the --version/-v flag prints exactly the bare version - matching
// buildinfo.Info.Short()'s contract of no program name, no commit, no
// date, no decoration. Exported so the fangcmd package can share the
// identical template rather than risk a second copy drifting from this
// one.
const VersionTemplate = "{{.Version}}\n"

// WireCobra adds a `version` subcommand to root (printing info.Long())
// and sets root.Version plus the version template so plain cobra's own
// --version/-v flag handling prints exactly info.Short() - no program
// name, no commit, no date, no decoration.
//
//	info := buildinfo.Get("mycli")
//	root := &cobra.Command{Use: "mycli"}
//	// ... attach the rest of the command tree to root ...
//	cobracmd.WireCobra(root, info)
//	if err := root.Execute(); err != nil {
//	    os.Exit(1)
//	}
//
// WireCobra does for plain cobra exactly what hand-rolling
// `root.Version = info.Short()` plus `root.SetVersionTemplate(...)` did
// in every consumer of this module before this helper existed - now both
// live in one tested place. It does not call root.Execute itself.
//
// A binary that is already built on fang should call fangcmd.Wire
// instead of WireCobra: the two package's version templates and
// VersionCommand are shared, so the flag and subcommand cannot disagree
// no matter which wiring path a given binary uses.
func WireCobra(root *cobra.Command, info buildinfo.Info) {
	root.AddCommand(VersionCommand(info))
	root.Version = info.Short()
	root.SetVersionTemplate(VersionTemplate)
}

// VersionCommand returns a standalone `version` subcommand that prints
// info.Long(). WireCobra (and fangcmd.Wire) add this automatically; call
// it directly only if you need to customize the command (e.g. change Use
// or Short) before adding it yourself.
func VersionCommand(info buildinfo.Info) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), info.Long())
			return err
		},
	}
}
