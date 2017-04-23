package main

import (
	"context"
	"os/exec"
)

// VCSInfoGit returns git facts for current dir or nil on error.
//
// More facts than wanted may be returned: some disabled wanted facts may
// be a required dependency for enabled wanted facts (this may also
// depends on opt), or implementation is just not optimized enough and get
// extra facts anyway (as side effect of gathering wanted facts).
func VCSInfoGit(ctx context.Context, wanted VCSInfoWanted, opt Options) *VCSInfo {
	facts := VCSInfo{VCS: VCSGit}
	if exec.Command("git", "status").Run() == nil {
		return &facts
	}
	// TODO
	return nil
}
