// Package cobracmd wires github.com/strongo/buildinfo into a cobra command
// tree fronted by charm.land/fang/v2 - the fleet's standard CLI stack.
//
// Left to its own defaults, fang resolves the --version/-v flag from
// runtime/debug.ReadBuildInfo() and prints "unknown (built from source)"
// whenever that lookup comes up empty, while a hand-rolled `version`
// subcommand elsewhere in the same binary reads an entirely different,
// ldflags-stamped value. That is the exact bug this package closes: Wire
// feeds fang and the `version` subcommand the same resolved
// buildinfo.Info, so the two surfaces cannot disagree.
package cobracmd

import (
	"fmt"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"
	"github.com/strongo/buildinfo"
)

// versionTemplate overrides cobra's default "<name> version <version>\n"
// so the --version/-v flag (handled by fang, via c.Version) prints exactly
// the bare version - matching buildinfo.Info.Short()'s contract of no
// program name, no commit, no date, no decoration.
const versionTemplate = "{{.Version}}\n"

// Wire adds a `version` subcommand to root (printing info.Long()) and
// returns the fang.Option(s) that make fang's --version/-v flag print
// exactly info.Short(). Pass the returned options straight into
// fang.Execute so both surfaces are driven by the one Info value:
//
//	info := buildinfo.Get("mycli")
//	root := &cobra.Command{Use: "mycli"}
//	// ... attach the rest of the command tree to root ...
//	if err := fang.Execute(ctx, root, cobracmd.Wire(root, info)...); err != nil {
//	    os.Exit(1)
//	}
//
// Wire does not call fang.Execute itself - the caller stays in control of
// any other fang.Option it wants (themes, signal handling, and so on).
func Wire(root *cobra.Command, info buildinfo.Info) []fang.Option {
	root.AddCommand(VersionCommand(info))

	// fang.Execute assigns root.Version from its own options every call, so
	// this template is what keeps that assignment from carrying fang's
	// default "<name> version " prefix or a "(shortsha)" suffix through to
	// output. It is read at print time, independent of when Version is set.
	root.SetVersionTemplate(versionTemplate)

	return []fang.Option{
		// Deliberately omits fang.WithCommit: fang appends "(<7-char-sha>)"
		// to the version string whenever a commit is supplied, which would
		// break Short()'s "exactly the version, no decoration" contract.
		fang.WithVersion(info.Short()),
	}
}

// VersionCommand returns a standalone `version` subcommand that prints
// info.Long(). Wire adds this to root automatically; call it directly only
// if you need to customize the command (e.g. change Use or Short) before
// adding it yourself.
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
