package main

import (
	"fmt"
	"log"
)

// Options contains common options provided by user.
// Different implementations may ignore some options.
type Options struct {
	DirtyIfUntracked    bool
	RenamesFromRewrites bool
	IncludeSubmodules   bool
}

// VCSInfoWanted lists facts requested by user.
type VCSInfoWanted struct {
	Revision              bool
	Branch                bool
	Tag                   bool
	Action                bool
	HasRemote             bool // imply Branch
	RemoteDivergedCommits bool // imply HasRemote
	HasStashedCommits     bool
	StashedCommits        bool // imply HasStashedCommits
	IsDirty               bool // may imply HasUntrackedFiles
	DirtyFiles            bool // do NOT imply IsDirty!
	HasRenamedFiles       bool // imply DirtyFiles, may be slow
	HasUntrackedFiles     bool // do NOT imply IsDirty!
}

// VCSInfo contains gathered facts about repo as general flat list, common
// for all VCS. For most VCS only some of these fields are actually
// provided - because not every field makes sense for every VCS, because
// of performance issues, or just because it wasn't implemented yet.
type VCSInfo struct {
	VCS                 VCSType
	RevisionShort       string
	Branch              string // Hg "bookmark"?
	Tag                 string // latest reachable from HEAD
	Action              VCSAction
	HasRemote           bool
	RemoteAheadCommits  int
	RemoteBehindCommits int
	HasStashedCommits   bool // only git?
	StashedCommits      int
	IsDirty             bool // Untracked? || Unmerged||Deleted||Renamed||Modified||Added
	HasAddedFiles       bool
	HasModifiedFiles    bool
	HasDeletedFiles     bool
	HasRenamedFiles     bool
	HasUnmergedFiles    bool // only git?
	AddedFiles          int
	ModifiedFiles       int
	DeletedFiles        int
	RenamedFiles        int
	UnmergedFiles       int
	HasUntrackedFiles   bool // too slow to even try to find how many
	// TODO Patch info
}

// Fix enforce consistent state between fields and reset fields not listed
// in wanted.
// It also output QA notice to note possible optimizations.
func (facts *VCSInfo) Fix(wanted VCSInfoWanted, opt Options) {
	if facts == nil {
		return
	}

	facts.HasRemote = facts.HasRemote ||
		facts.RemoteAheadCommits != 0 || facts.RemoteBehindCommits != 0
	facts.HasStashedCommits = facts.HasStashedCommits || facts.StashedCommits != 0
	facts.HasAddedFiles = facts.HasAddedFiles || facts.AddedFiles != 0
	facts.HasModifiedFiles = facts.HasModifiedFiles || facts.ModifiedFiles != 0
	facts.HasDeletedFiles = facts.HasDeletedFiles || facts.DeletedFiles != 0
	facts.HasRenamedFiles = facts.HasRenamedFiles || facts.RenamedFiles != 0
	facts.HasUnmergedFiles = facts.HasUnmergedFiles || facts.UnmergedFiles != 0
	facts.IsDirty = facts.IsDirty ||
		facts.HasAddedFiles || facts.HasModifiedFiles || facts.HasDeletedFiles ||
		facts.HasRenamedFiles || facts.HasUnmergedFiles ||
		(facts.HasUntrackedFiles && opt.DirtyIfUntracked)

	if !wanted.Revision && facts.RevisionShort != "" {
		log.Print("QA notice: unwanted RevisionShort")
		facts.RevisionShort = ""
	}
	if !wanted.Branch && !wanted.HasRemote && !wanted.RemoteDivergedCommits && facts.Branch != "" {
		log.Print("QA notice: unwanted Branch")
		facts.Branch = ""
	}
	if !wanted.Tag && facts.Tag != "" {
		log.Print("QA notice: unwanted Tag")
		facts.Tag = ""
	}
	if !wanted.Action && facts.Action != 0 {
		log.Print("QA notice: unwanted Action")
		facts.Action = 0
	}
	if !wanted.HasRemote && !wanted.RemoteDivergedCommits && facts.HasRemote == true {
		log.Print("QA notice: unwanted HasRemote")
		facts.HasRemote = false
	}
	if !wanted.RemoteDivergedCommits && (facts.RemoteAheadCommits != 0 || facts.RemoteBehindCommits != 0) {
		log.Print("QA notice: unwanted Remote(Ahead|Behind)Commits")
		facts.RemoteAheadCommits = 0
		facts.RemoteBehindCommits = 0
	}
	if !wanted.HasStashedCommits && !wanted.StashedCommits && facts.HasStashedCommits {
		log.Print("QA notice: unwanted HasStashedCommits")
		facts.HasStashedCommits = false
	}
	if !wanted.StashedCommits && facts.StashedCommits != 0 {
		log.Print("QA notice: unwanted StashedCommits")
		facts.StashedCommits = 0
	}
	if !wanted.IsDirty && !(wanted.HasUntrackedFiles && opt.DirtyIfUntracked) &&
		!(wanted.DirtyFiles || wanted.HasRenamedFiles) && facts.IsDirty {
		log.Print("QA notice: unwanted IsDirty")
		facts.IsDirty = false
	}
	if !wanted.HasRenamedFiles && (facts.HasRenamedFiles || facts.RenamedFiles != 0) {
		log.Print("QA notice: unwanted (Has)?RenamedFiles")
		facts.HasRenamedFiles = false
		facts.RenamedFiles = 0
	}
	if !(wanted.DirtyFiles || wanted.HasRenamedFiles) &&
		(facts.HasAddedFiles || facts.AddedFiles != 0 ||
			facts.HasModifiedFiles || facts.ModifiedFiles != 0 ||
			facts.HasDeletedFiles || facts.DeletedFiles != 0 ||
			facts.HasUnmergedFiles || facts.UnmergedFiles != 0) {
		log.Print("QA notice: unwanted (Has)?(Added|Modified|Deleted|Unmerged)Files")
		facts.HasAddedFiles = false
		facts.HasModifiedFiles = false
		facts.HasDeletedFiles = false
		facts.HasUnmergedFiles = false
		facts.AddedFiles = 0
		facts.ModifiedFiles = 0
		facts.DeletedFiles = 0
		facts.UnmergedFiles = 0
	}
	if !wanted.HasUntrackedFiles && !(wanted.IsDirty && opt.DirtyIfUntracked) && facts.HasUntrackedFiles {
		log.Print("QA notice: unwanted HasUntrackedFiles")
		facts.HasUntrackedFiles = false
	}
}

// VCSType is VCS enumeration.
type VCSType int

const (
	// Supported VCS types.
	VCSGit VCSType = iota + 1
	VCSMercurial
)

// String returns text representation for VCS type.
func (name VCSType) String() string {
	// TODO Use user-provided values (in env variable(s)?).
	switch name {
	case VCSGit:
		return "git"
	case VCSMercurial:
		return "hg"
	default:
		panic(fmt.Sprintf("unknown VCSType: %d", name))
	}
}

// VCSAction is VCS state (merge conflict, interactive rebase, …) enumeration.
type VCSAction int

const (
	// Current repo state/action (most probably git-specific).
	ActionNone VCSAction = iota
	ActionMerge
	ActionRevert
	ActionCherrypick
	ActionBisect
	ActionRebase
	ActionRebaseInteractive
	ActionRebaseMerge
	ActionApplyMailbox
	ActionApplyMailboxOrRebase
)

// String returns text representation for VCS action/state.
func (action VCSAction) String() string {
	// TODO Use user-provided values (in env variable(s)?).
	switch action {
	case ActionNone:
		return ""
	case ActionMerge:
		return "merge"
	case ActionRevert:
		return "revert"
	case ActionCherrypick:
		return "cherry"
	case ActionBisect:
		return "bisect"
	case ActionRebase:
		return "rebase"
	case ActionRebaseInteractive:
		return "rebase-i"
	case ActionRebaseMerge:
		return "rebase-m"
	case ActionApplyMailbox:
		return "am"
	case ActionApplyMailboxOrRebase:
		return "am/rebase"
	default:
		panic(fmt.Sprintf("unknown VCSAction: %d", action))
	}
}

// Goals:
// - vcprompt drop-in replacement mode
//   * rename binary in GH releases? move to cmd/vcprompt/?
// - provide much more facts - like oh-my-zsh
// - 100% user-configurable result
//   * to split facts into PS1/RPS1 - just output list with all facts to
//     STDOUT and let user handle it
//   * for simpler case output single string in vcprompt-like way but let
//     user define actual text for each fact (in environment, or check how
//     other vcprompt forks do this) - it may be useful to provide
//     optional non-empty actual text for absent fact, as a poor man's "if"
// - may use mix of fast-internal implementations and exec of external
//   commands, and mix of fast-inaccurate and slow-accurate facts
//   * mark inaccurate facts somehow, to make it possible to gradually
//     improve without breaking compatibility: accurate facts
//     implementation may be added later or replaced by faster accurate
//     implementation, inaccurate implementation may be added later (even
//     more than one) or replaced by faster and/or accurate one
// - context configuration like vcs-info
//   * maybe outside of this tool, in zsh prompt code
//   * don't use git-config to store own settings
//   * per-repo and per-VCS
//   * fast but not 100% accurate or slow but correct
// - be as fast as possible
//   * try to avoid executing external commands
//   * try to use multiple CPU cores for parallel tasks
//   * if multiple external commands to be executed - run them in parallel
//   * do not gather facts not requested by user
// - fast and feature-rich for git main facts from v1.0
// - hg support based on vcprompt-hgst from v2.0
// - improve hg support (features and/or speed) if possible
// - bzr/svn/etc. support blindly copied from other tools when possible
func main() {
	// TODO
	// - parse flags
	//   * output help
	//   * setup log to STDERR only in debug mode, otherwise to /dev/null
	// - detect VCS here, to avoid trying different VCS engines
	//   one-by-one and have each one read same dirs again and again
	//   * chdir to repo root, to avoid more detection by executed
	//     commands and simplify rest of code
	// - exit without output if no VCS detected
	// - get rest of configuration (from environment?)
	// - call VCS engine to gather facts according to configuration
	// - output facts in user-defined format
}
