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
// More facts than requested may be returned: some non-requested facts may
// be a required dependency for requested facts (this may also depends on
// requested options), or implementation is just not optimized enough and
// get extra facts anyway (as side effect of gathering requested facts).
func VCSInfoGit(ctx context.Context, facts *Facts) {
	// XXX Do not call Free() on any object - this should be faster
	// and safe for short-lived command. DO NOT COPY&PASTE THIS AS IS!

	// TODO Run everything in goroutines, and receive detected facts
	// from then using channels. Return facts collected so far if context
	// will be canceled because of deadline before all goroutines finish.

	// Honor requested attributes.
	facts.Lookup = facts.Req.Attr
	l := &facts.Lookup

	// Needs to lookup for dependent attributes too.
	// Has*Files isn't marked as a dependency for IsDirty because this
	// will break later optimization when scanning for files.
	l.HasUnmergedFiles = l.HasUnmergedFiles || l.UnmergedFiles
	l.HasRenamedFiles = l.HasRenamedFiles || l.RenamedFiles
	l.HasDeletedFiles = l.HasDeletedFiles || l.DeletedFiles
	l.HasModifiedFiles = l.HasModifiedFiles || l.ModifiedFiles
	l.HasAddedFiles = l.HasAddedFiles || l.AddedFiles
	l.HasStashedCommits = l.HasStashedCommits || l.StashedCommits
	l.HasRemote = l.HasRemote || l.CommitsAheadRemote || l.CommitsBehindRemote
	l.Branch = l.Branch || l.HasRemote
	l.VCS = true

	repo, err := git2go.OpenRepository(".")
	if err != nil {
		return // not a (valid) repo
	}
	head, _ := repo.Head() // err if empty repo without commits
	facts.Found.VCS = VCSGit

	if l.RevisionShort && head != nil {
		facts.Found.RevisionShort = head.Target().String()[:7]
	}

	var branch *git2go.Branch
	if l.Branch && head != nil {
		if branch = head.Branch(); branch != nil {
			facts.Found.Branch, _ = branch.Name() // err if detached HEAD
		}
	}

	if l.Tag {
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
			return // FIXME just skip l.Tag?
		}
		err = revwalk.PushHead()
		if err != nil {
			log.Println("revwalk.PushHead:", err)
			return // FIXME just skip l.Tag?
		}
		var id git2go.Oid
		for facts.Found.Tag == "" {
			err = revwalk.Next(&id)
			if git2go.IsErrorCode(err, git2go.ErrIterOver) {
				break
			}
			if err != nil {
				log.Println("revwalk.Next:", err)
				return // FIXME just skip l.Tag?
			}
			if tags[id] != nil {
				facts.Found.Tag = tags[id].Name()
			}
		}
	}

	if l.State {
		switch state := repo.State(); state {
		case git2go.RepositoryStateNone:
			facts.Found.State = StateNone
		case git2go.RepositoryStateMerge:
			facts.Found.State = StateMerge
		case git2go.RepositoryStateRevert:
			facts.Found.State = StateRevert
		case git2go.RepositoryStateCherrypick:
			facts.Found.State = StateCherrypick
		case git2go.RepositoryStateBisect:
			facts.Found.State = StateBisect
		case git2go.RepositoryStateRebase:
			facts.Found.State = StateRebase
		case git2go.RepositoryStateRebaseInteractive:
			facts.Found.State = StateRebaseInteractive
		case git2go.RepositoryStateRebaseMerge:
			facts.Found.State = StateRebaseMerge
		case git2go.RepositoryStateApplyMailbox:
			facts.Found.State = StateApplyMailbox
		case git2go.RepositoryStateApplyMailboxOrRebase:
			facts.Found.State = StateApplyMailboxOrRebase
		default:
			log.Println("repo.State unknown:", state)
		}
	}

	if l.HasRemote && branch != nil {
		upstream, err := branch.Upstream()
		facts.Found.HasRemote = err == nil
		if (l.CommitsAheadRemote || l.CommitsBehindRemote) && facts.Found.HasRemote {
			// TODO run as goroutine - walk commits and calculate distance
			facts.Found.CommitsAheadRemote, facts.Found.CommitsBehindRemote, err =
				repo.AheadBehind(branch.Target(), upstream.Target())
			if err != nil {
				log.Println("repo.AheadBehind:", err)
				return // FIXME just skip l.HasRemote
			}
		}
	}

	if l.HasStashedCommits {
		repo.Stashes.Foreach(func(index int, message string, id *git2go.Oid) error {
			facts.Found.HasStashedCommits = true
			if !l.StashedCommits {
				return errors.New("done")
			}
			facts.Found.StashedCommits++
			return nil
		})
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
	var countFiles = l.AddedFiles || l.ModifiedFiles || l.DeletedFiles ||
		l.RenamedFiles || l.UnmergedFiles
	switch {
	case l.HasModifiedFiles, l.HasDeletedFiles, l.HasRenamedFiles, l.HasUnmergedFiles:
		tryStatusShow = append(tryStatusShow, git2go.StatusShowIndexAndWorkdir)
	case l.HasAddedFiles && l.HasUntrackedFiles:
		tryStatusShow = append(tryStatusShow, git2go.StatusShowIndexAndWorkdir)
	case l.IsDirty && l.HasAddedFiles:
		tryStatusShow = append(tryStatusShow, git2go.StatusShowIndexAndWorkdir)
	case l.IsDirty && l.HasUntrackedFiles:
		tryStatusShow = append(tryStatusShow, git2go.StatusShowWorkdirOnly)
		tryStatusShow = append(tryStatusShow, git2go.StatusShowIndexOnly)
	case l.IsDirty:
		tryStatusShow = append(tryStatusShow, git2go.StatusShowIndexOnly)
		tryStatusShow = append(tryStatusShow, git2go.StatusShowWorkdirOnly)
	case l.HasAddedFiles:
		tryStatusShow = append(tryStatusShow, git2go.StatusShowIndexOnly)
	case l.HasUntrackedFiles:
		tryStatusShow = append(tryStatusShow, git2go.StatusShowWorkdirOnly)
	}
StatusList:
	for _, statusShow := range tryStatusShow {
		var statusOpt git2go.StatusOpt
		if statusShow != git2go.StatusShowIndexOnly {
			statusOpt |= git2go.StatusOptUpdateIndex
			if l.HasUntrackedFiles || (l.IsDirty && facts.Req.DirtyIfUntracked) {
				statusOpt |= git2go.StatusOptIncludeUntracked
			}
		}
		if l.HasRenamedFiles {
			statusOpt |= git2go.StatusOptRenamesHeadToIndex
			statusOpt |= git2go.StatusOptRenamesIndexToWorkdir
			if facts.Req.RenamesFromRewrites {
				statusOpt |= git2go.StatusOptRenamesFromRewrites
			}
		}
		if !facts.Req.IncludeSubmodules {
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
				facts.Found.AddedFiles++
				facts.Found.HasAddedFiles = true
			}
			if entry.Status&(git2go.StatusIndexModified|git2go.StatusWtModified) > 0 {
				entry.Status &^= git2go.StatusIndexModified | git2go.StatusWtModified
				facts.Found.ModifiedFiles++
				facts.Found.HasModifiedFiles = true
			}
			if entry.Status&(git2go.StatusIndexDeleted|git2go.StatusWtDeleted) > 0 {
				entry.Status &^= git2go.StatusIndexDeleted | git2go.StatusWtDeleted
				facts.Found.DeletedFiles++
				facts.Found.HasDeletedFiles = true
			}
			if entry.Status&(git2go.StatusIndexRenamed|git2go.StatusWtRenamed) > 0 {
				entry.Status &^= git2go.StatusIndexRenamed | git2go.StatusWtRenamed
				facts.Found.RenamedFiles++
				facts.Found.HasRenamedFiles = true
			}
			if entry.Status&(git2go.StatusIndexTypeChange|git2go.StatusWtTypeChange) > 0 { // file/symlink
				entry.Status &^= git2go.StatusIndexTypeChange | git2go.StatusWtTypeChange
				facts.Found.ModifiedFiles++
				facts.Found.HasModifiedFiles = true
			}
			switch entry.Status {
			case 0: // was bitmask flag(s)
			case git2go.StatusWtNew:
				facts.Found.HasUntrackedFiles = true
			case git2go.StatusIgnored:
				continue
			case git2go.StatusConflicted:
				// TODO According to git-status(1) "unmerged"
				// means some combinations of Index/Wt statuses:
				//   StatusIndex…  StatusWt…
				//     Deleted       Deleted
				//     New           Updated?
				//     Updated?      Deleted
				//     Updated?      New
				//     Deleted       Updated?
				//     New           New
				//     Updated?      Updated?
				// What is "Updated?" isn't clear - looks like
				// similar to Modified, but not exactly (because
				//     New           Modified
				// is usual case have nothing with "unmerged").
				// Maybe entry.Status=StatusConflicted and these
				// combinations came from two other flags:
				//   entry.HeadToIndex.Status
				//   entry.IndexToWorkdir.Status
				// which have similar (Add/Mod/Del/…) values.
				facts.Found.UnmergedFiles++
				facts.Found.HasUnmergedFiles = true
			default:
				// May be "unreadable" - this const wasn't
				// imported from C, not sure why.
				log.Println("entry.Status unknown:", entry.Status, entry)
				continue
			}
			// if we here, then something is changed!
			if countFiles {
				continue
			}
			if (!l.IsDirty ||
				facts.Found.HasAddedFiles || facts.Found.HasModifiedFiles ||
				facts.Found.HasDeletedFiles || facts.Found.HasRenamedFiles ||
				facts.Found.HasUnmergedFiles ||
				(facts.Req.DirtyIfUntracked && facts.Found.HasUntrackedFiles)) &&
				(!l.HasAddedFiles || facts.Found.HasAddedFiles) &&
				(!l.HasModifiedFiles || facts.Found.HasModifiedFiles) &&
				(!l.HasDeletedFiles || facts.Found.HasDeletedFiles) &&
				(!l.HasRenamedFiles || facts.Found.HasRenamedFiles) &&
				(!l.HasUnmergedFiles || facts.Found.HasUnmergedFiles) &&
				(!l.HasUntrackedFiles || facts.Found.HasUntrackedFiles) {
				// Break early, reset incomplete counters.
				facts.Found.AddedFiles = 0
				facts.Found.ModifiedFiles = 0
				facts.Found.DeletedFiles = 0
				facts.Found.RenamedFiles = 0
				facts.Found.UnmergedFiles = 0
				break StatusList
			}
		}
	}
	// Update facts.Lookup to actual dependencies to avoid
	// false-positive QA notices.
	for _, statusShow := range tryStatusShow {
		l.HasModifiedFiles = true
		l.ModifiedFiles = true
		l.HasDeletedFiles = true
		l.DeletedFiles = true
		l.HasRenamedFiles = true
		l.RenamedFiles = true
		switch statusShow {
		case git2go.StatusShowIndexOnly:
			l.HasAddedFiles = true
			l.AddedFiles = true
		case git2go.StatusShowWorkdirOnly:
			l.HasUntrackedFiles = l.HasUntrackedFiles || (l.IsDirty && facts.Req.DirtyIfUntracked)
		case git2go.StatusShowIndexAndWorkdir:
			l.HasAddedFiles = true
			l.AddedFiles = true
			l.HasUnmergedFiles = true
			l.UnmergedFiles = true
			l.HasUntrackedFiles = l.HasUntrackedFiles || (l.IsDirty && facts.Req.DirtyIfUntracked)
		}
	}
}
