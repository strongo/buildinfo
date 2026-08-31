package buildinfo

import "runtime/debug"

// fakeBuildInfo constructs a *debug.BuildInfo shaped the way
// runtime/debug.ReadBuildInfo() would for a program built from git, so
// applyBuildInfo can be tested without depending on this test binary's own
// (unreliable, per the package doc) build info.
func fakeBuildInfo(mainVersion, revision, vcsTime string, modified bool) *debug.BuildInfo {
	modifiedStr := "false"
	if modified {
		modifiedStr = "true"
	}
	return &debug.BuildInfo{
		Main: debug.Module{
			Version: mainVersion,
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs", Value: "git"},
			{Key: "vcs.revision", Value: revision},
			{Key: "vcs.time", Value: vcsTime},
			{Key: "vcs.modified", Value: modifiedStr},
		},
	}
}
