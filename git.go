package main

import (
	"context"
	"errors"
	"log"

	git2go "github.com/libgit2/git2go"
)

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
		facts.RevisionShort = head.Target().String()[:7]
	}
	var branch *git2go.Branch
	if wanted.Branch {
		if branch = head.Branch(); branch != nil {
			facts.Branch, _ = branch.Name() // err if detached HEAD
		}
	}
	if wanted.Tag {
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
			return nil // FIXME just skip wanted.Tag?
		}
		err = revwalk.PushHead()
		if err != nil {
			log.Println("revwalk.PushHead:", err)
			return nil // FIXME just skip wanted.Tag?
		}
		var id git2go.Oid
		for facts.Tag == "" {
			err = revwalk.Next(&id)
			if git2go.IsErrorCode(err, git2go.ErrIterOver) {
				break
			}
			if err != nil {
				log.Println("revwalk.Next:", err)
				return nil // FIXME just skip wanted.Tag?
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
			facts.RemoteAheadCommits, facts.RemoteBehindCommits, err =
				repo.AheadBehind(branch.Target(), upstream.Target())
			if err != nil {
				log.Println("repo.AheadBehind:", err)
				return nil // FIXME just skip wanted.HasRemote
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
	return facts
}
