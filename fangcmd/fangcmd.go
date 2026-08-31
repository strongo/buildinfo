// Package fangcmd wires github.com/strongo/buildinfo into a cobra command
// tree fronted by charm.land/fang/v2 - the fleet's standard CLI stack.
//
// Left to its own defaults, fang resolves the --version/-v flag from
// runtime/debug.ReadBuildInfo() and prints "unknown (built from source)"
// whenever that lookup comes up empty, while a hand-rolled `version`
// subcommand elsewhere in the same binary reads an entirely different,
// ldflags-stamped value. That is the exact bug this package closes: Wire
// feeds fang and the `version` subcommand the same resolved
// buildinfo.Info, so the two surfaces cannot disagree.
//
// Only import this package when the binary already depends on fang -
// importing it links fang and its terminal-UI dependencies into the
// binary. A binary that just wants the --version flag and a `version`
// subcommand wired onto plain cobra, without paying that cost, should
// import github.com/strongo/buildinfo/cobracmd instead; that package
// never imports fang.
package fangcmd

import (
	"charm.land/fang/v2"
	"github.com/spf13/cobra"

	"github.com/strongo/buildinfo"
	"github.com/strongo/buildinfo/cobracmd"
)

// Wire adds a `version` subcommand to root (printing info.Long()) and
// returns the fang.Option(s) that make fang's --version/-v flag print
// exactly info.Short(). Pass the returned options straight into
// fang.Execute so both surfaces are driven by the one Info value:
//
//	info := buildinfo.Get("mycli")
//	root := &cobra.Command{Use: "mycli"}
//	// ... attach the rest of the command tree to root ...
//	if err := fang.Execute(ctx, root, fangcmd.Wire(root, info)...); err != nil {
//	    os.Exit(1)
//	}
//
// Wire does not call fang.Execute itself - the caller stays in control of
// any other fang.Option it wants (themes, signal handling, and so on).
// It adds the same cobracmd.VersionCommand and uses the same
// cobracmd.VersionTemplate as cobracmd.WireCobra, so a binary cannot get
// a different answer from --version and `version` no matter which of the
// two wiring paths (this one, or plain-cobra WireCobra) it uses.
func Wire(root *cobra.Command, info buildinfo.Info) []fang.Option {
	root.AddCommand(cobracmd.VersionCommand(info))

	// fang.Execute assigns root.Version from its own options every call, so
	// this template is what keeps that assignment from carrying fang's
	// default "<name> version " prefix or a "(shortsha)" suffix through to
	// output. It is read at print time, independent of when Version is set.
	root.SetVersionTemplate(cobracmd.VersionTemplate)

	return []fang.Option{
		// Deliberately omits fang.WithCommit: fang appends "(<7-char-sha>)"
		// to the version string whenever a commit is supplied, which would
		// break Short()'s "exactly the version, no decoration" contract.
		fang.WithVersion(info.Short()),
	}
}
