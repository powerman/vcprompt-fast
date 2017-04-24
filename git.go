package main

import (
	"context"
	"errors"
	"log"

	git2go "github.com/libgit2/git2go"
)

// Consider different implementations:
// - github.com/libgit2/git2go
//   * wrapper for C libgit2
//   * require --tags=static to build
//   * require -buildmode=pie OR paxctl-ng -m on GrSecurity
//   * looks like it doesn't support --untracked-cache (core.untrackedCache)
//     and thus may be slower than `git status --porcelain` for untracked
//     files detection (if user has manually enabled this cache!)
//   * no(?) way for really low-level control to avoid redundant processing
//     for workdir files
// - https://godoc.org/gopkg.in/src-d/go-git.v4
//   * pure Go
// - https://godoc.org/github.com/speedata/gogit
//   https://godoc.org/github.com/gogits/git
//   https://godoc.org/github.com/sourcegraph/go-git
//   * pure Go, unmaintained diverse clones
// - `git status --porcelain`
//   * support --untracked-cache (core.untrackedCache)
//   * several exec may be needed to get all required info
// - some combination of above implementations

// VCSInfoGit returns git facts for current dir or nil on error.
//
// More facts than wanted may be returned: some disabled wanted facts may
// be a required dependency for enabled wanted facts (this may also
// depends on opt), or implementation is just not optimized enough and get
// extra facts anyway (as side effect of gathering wanted facts).
func VCSInfoGit(ctx context.Context, wanted VCSInfoWanted, opt Options) *VCSInfo {
	wanted.HasRemote = wanted.RemoteDivergedCommits || wanted.HasRemote
	wanted.Branch = wanted.HasRemote || wanted.Branch
	wanted.HasStashedCommits = wanted.StashedCommits || wanted.HasStashedCommits

	facts := &VCSInfo{VCS: VCSGit}
	// XXX Do not call Free() on any object - this should be faster
	// and safe for short-lived command. DO NOT COPY&PASTE THIS AS IS!
	repo, err := git2go.OpenRepository(".")
	if err != nil {
		return nil // not a (valid) repo?
	}
	head, err := repo.Head()
	if err != nil {
		return facts // empty repo without commits?
	}
	if wanted.Revision {
		// TODO try Shorthand() instead of [:7]?
		facts.RevisionShort = head.Target().String()[:7]
	}
	var branch *git2go.Branch
	if wanted.Branch {
		if branch = head.Branch(); branch != nil {
			facts.Branch, _ = branch.Name() // err if detached HEAD
		}
	}
	if wanted.Tag {
		// TODO run as goroutine - may walk all commits
		tags := make(map[git2go.Oid]*git2go.Tag)
		repo.Tags.Foreach(func(name string, id *git2go.Oid) error {
			tag, err := repo.LookupTag(id)
			if err != nil {
				return nil // non-annotated tag
			}
			obj, err := tag.Peel(git2go.ObjectCommit)
			if err != nil {
				log.Println("tag.Peel:", err)
				return nil
			}
			id = obj.Id()
			if tags[*id] == nil || tags[*id].Tagger().When.Before(tag.Tagger().When) {
				tags[*id] = tag
			}
			return nil
		})
		revwalk, err := repo.Walk()
		if err != nil {
			log.Println("repo.Walk:", err)
			return facts // FIXME just skip wanted.Tag?
		}
		err = revwalk.PushHead()
		if err != nil {
			log.Println("revwalk.PushHead:", err)
			return facts // FIXME just skip wanted.Tag?
		}
		var id git2go.Oid
		for facts.Tag == "" {
			err = revwalk.Next(&id)
			if git2go.IsErrorCode(err, git2go.ErrIterOver) {
				break
			}
			if err != nil {
				log.Println("revwalk.Next:", err)
				return facts // FIXME just skip wanted.Tag?
			}
			if tags[id] != nil {
				facts.Tag = tags[id].Name()
			}
		}
	}
	if wanted.HasRemote && branch != nil {
		upstream, err := branch.Upstream()
		facts.HasRemote = err == nil
		if wanted.RemoteDivergedCommits && facts.HasRemote {
			// TODO run as goroutine - walk commits and calculate distance
			facts.RemoteAheadCommits, facts.RemoteBehindCommits, err =
				repo.AheadBehind(branch.Target(), upstream.Target())
			if err != nil {
				log.Println("repo.AheadBehind:", err)
				return facts // FIXME just skip wanted.HasRemote
			}
		}
	}
	if wanted.HasStashedCommits {
		repo.Stashes.Foreach(func(index int, message string, id *git2go.Oid) error {
			facts.HasStashedCommits = true
			if !wanted.StashedCommits {
				return errors.New("done")
			}
			facts.StashedCommits++
			return nil
		})
	}
	if wanted.Action {
		switch state := repo.State(); state {
		case git2go.RepositoryStateNone:
			facts.Action = ActionNone
		case git2go.RepositoryStateMerge:
			facts.Action = ActionMerge
		case git2go.RepositoryStateRevert:
			facts.Action = ActionRevert
		case git2go.RepositoryStateCherrypick:
			facts.Action = ActionCherrypick
		case git2go.RepositoryStateBisect:
			facts.Action = ActionBisect
		case git2go.RepositoryStateRebase:
			facts.Action = ActionRebase
		case git2go.RepositoryStateRebaseInteractive:
			facts.Action = ActionRebaseInteractive
		case git2go.RepositoryStateRebaseMerge:
			facts.Action = ActionRebaseMerge
		case git2go.RepositoryStateApplyMailbox:
			facts.Action = ActionApplyMailbox
		case git2go.RepositoryStateApplyMailboxOrRebase:
			facts.Action = ActionApplyMailboxOrRebase
		default:
			log.Println("repo.State unknown:", state)
		}
	}
	// Questionable optimizations (TBD):
	// - Parallelize processing of status entries for large EntryCount() -
	//   but how many files should be modifed/untracked/etc. to worth it?
	// - Try to detect IsDirty on workdir without StatusOptIncludeUntracked
	//   first, and if IsDirty still false then try again with it (double
	//   scan which may be faster if there are a lot of untracked files and
	//   not so many tracked files - unlikely case, at a glance)
	// - Try to use Pathspec with StatusOptDisablePathspecMatch to check
	//   each top level file/dir one-by-one manually - this may helps detect
	//   IsDirty and/or HasUntrackedFiles without full scan, but this will
	//   break renames detection, may break StatusOptUpdateIndex, in case
	//   most of files are in single top-level subdir this may require using
	//   subdirectories as some depth instead of top-level files/dirs only
	var tryStatusShow []git2go.StatusShow
	if !wanted.DirtyFiles && !wanted.HasRenamedFiles && !wanted.HasUntrackedFiles {
		if wanted.IsDirty {
			tryStatusShow = []git2go.StatusShow{
				git2go.StatusShowIndexOnly, git2go.StatusShowWorkdirOnly}
		}
	} else {
		tryStatusShow = []git2go.StatusShow{git2go.StatusShowIndexAndWorkdir}
	}
StatusList:
	for _, statusShow := range tryStatusShow {
		var statusOpt git2go.StatusOpt
		if statusShow != git2go.StatusShowIndexOnly {
			statusOpt |= git2go.StatusOptUpdateIndex
			if wanted.HasUntrackedFiles || (wanted.IsDirty && opt.DirtyIfUntracked) {
				statusOpt |= git2go.StatusOptIncludeUntracked
			}
		}
		if wanted.HasRenamedFiles {
			statusOpt |= git2go.StatusOptRenamesHeadToIndex
			statusOpt |= git2go.StatusOptRenamesIndexToWorkdir
			if opt.RenamesFromRewrites {
				statusOpt |= git2go.StatusOptRenamesFromRewrites
			}
		}
		if !opt.IncludeSubmodules {
			statusOpt |= git2go.StatusOptExcludeSubmodules
		}
		statuses, err := repo.StatusList(&git2go.StatusOptions{
			Show:     statusShow,
			Flags:    statusOpt,
			Pathspec: nil,
		})
		if err != nil {
			log.Println("repo.StatusList:", err)
			continue
		}
		n, err := statuses.EntryCount()
		if err != nil {
			log.Println("statuses.EntryCount:", err)
			continue
		}
		for i := 0; i < n; i++ {
			entry, err := statuses.ByIndex(i)
			if err != nil {
				log.Println("statuses.ByIndex:", err)
				continue StatusList
			}
			if entry.Status == git2go.StatusCurrent { // == 0, so must check it first
				// never here: StatusOptIncludeUnmodified wasn't enabled
				log.Println("entry.Status=unmodified")
				continue
			}
			if entry.Status&git2go.StatusIndexNew > 0 {
				entry.Status &^= git2go.StatusIndexNew
				facts.AddedFiles++
			}
			if entry.Status&(git2go.StatusIndexModified|git2go.StatusWtModified) > 0 {
				entry.Status &^= git2go.StatusIndexModified | git2go.StatusWtModified
				facts.ModifiedFiles++
			}
			if entry.Status&(git2go.StatusIndexDeleted|git2go.StatusWtDeleted) > 0 {
				entry.Status &^= git2go.StatusIndexDeleted | git2go.StatusWtDeleted
				facts.DeletedFiles++
			}
			if entry.Status&(git2go.StatusIndexRenamed|git2go.StatusWtRenamed) > 0 {
				entry.Status &^= git2go.StatusIndexRenamed | git2go.StatusWtRenamed
				facts.RenamedFiles++
			}
			if entry.Status&(git2go.StatusIndexTypeChange|git2go.StatusWtTypeChange) > 0 { // file/dir?
				entry.Status &^= git2go.StatusIndexTypeChange | git2go.StatusWtTypeChange
				facts.ModifiedFiles++
			}
			switch entry.Status {
			case 0: // was bitmask flag(s)
			case git2go.StatusWtNew:
				facts.HasUntrackedFiles = true
				if !opt.DirtyIfUntracked {
					continue
				}
			case git2go.StatusIgnored:
				continue
			case git2go.StatusConflicted:
				facts.UnmergedFiles++
			default:
				log.Println("entry.Status unknown:", entry.Status, entry)
				continue
			}
			// if we here, then it's dirty!
			if statusShow == git2go.StatusShowIndexOnly {
				break StatusList
			}
		}
	}
	return facts
}
