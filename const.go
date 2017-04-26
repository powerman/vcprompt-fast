package main

import "fmt"

// VCSType is VCS enumeration.
type VCSType int

// Supported VCS types.
const (
	VCSNone VCSType = iota
	VCSGit
	VCSMercurial
)

// String returns text representation for VCS type.
func (name VCSType) String() string {
	switch name {
	case VCSNone:
		return ""
	case VCSGit:
		return "git"
	case VCSMercurial:
		return "hg"
	default:
		panic(fmt.Sprintf("unknown VCSType: %d", name))
	}
}

// VCSState is VCS state (merge conflict, interactive rebase, …) enumeration.
type VCSState int

// Current repo state (most probably git-specific).
const (
	StateNone VCSState = iota
	StateMerge
	StateRevert
	StateCherrypick
	StateBisect
	StateRebase
	StateRebaseInteractive
	StateRebaseMerge
	StateApplyMailbox
	StateApplyMailboxOrRebase
)

// String returns text representation for VCS state.
func (state VCSState) String() string {
	switch state {
	case StateNone:
		return ""
	case StateMerge:
		return "merge"
	case StateRevert:
		return "revert"
	case StateCherrypick:
		return "cherry"
	case StateBisect:
		return "bisect"
	case StateRebase:
		return "rebase"
	case StateRebaseInteractive:
		return "rebase-i"
	case StateRebaseMerge:
		return "rebase-m"
	case StateApplyMailbox:
		return "am"
	case StateApplyMailboxOrRebase:
		return "am/rebase"
	default:
		panic(fmt.Sprintf("unknown VCSState: %d", state))
	}
}
