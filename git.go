package main

import (
	"context"
	"os/exec"
)

// VCSInfoGit returns git facts for current dir or nil on error.
func VCSInfoGit(ctx context.Context, wanted VCSInfoWanted, opt Options) *VCSInfo {
	facts := VCSInfo{VCS: VCSGit}
	if exec.Command("git", "status").Run() == nil {
		return &facts
	}
	// TODO
	return nil
}
