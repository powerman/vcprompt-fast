package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	. "gopkg.in/check.v1"
)

func git(args string) {
	cmd := exec.Command("git", strings.Split(args, " ")...)
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	if strings.HasPrefix(args, "tag ") {
		time.Sleep(time.Second) // make sure next tag will have later time
	}
}

// for debugging tests
func gitstatus() {
	cmd := exec.Command("git", "status", "--short")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func gitconfig() {
	git("config user.email root@localhost")
	git("config user.name Nobody")
	git("config commit.gpgsign false") // in case ~/.gitconfig contains true
}

type GitSuite struct {
	origDir string
	ctx     context.Context
	req     Request
	res     Attr
	want    Attr
}

var _ = Suite(&GitSuite{})

func (s *GitSuite) VCSInfoGit() Attr {
	facts := Facts{Req: s.req}
	VCSInfoGit(s.ctx, &facts)
	facts.QA()
	s.res = facts.Result()
	return s.res
}

func (s *GitSuite) enable_possible_optimizations() {
	// IsDirty and DirtyIfUntracked are excluded from this list
	// because they sometimes needs to be set in test wrapper func,
	// before call to this func.
	s.req.Attr.HasAddedFiles = false
	s.req.Attr.AddedFiles = false
	s.req.Attr.HasModifiedFiles = false
	s.req.Attr.ModifiedFiles = false
	s.req.Attr.HasDeletedFiles = false
	s.req.Attr.DeletedFiles = false
	s.req.Attr.HasRenamedFiles = false
	s.req.Attr.RenamedFiles = false
	s.req.Attr.HasUnmergedFiles = false
	s.req.Attr.UnmergedFiles = false
	s.req.Attr.HasUntrackedFiles = false
}

func (s *GitSuite) SetUpSuite(c *C) {
	s.ctx = context.Background()
	var err error
	s.origDir, err = os.Getwd()
	c.Assert(err, IsNil)
}

func (s *GitSuite) SetUpTest(c *C) {
	// These defaults make it easier to compare full Attr.
	s.req = Request{
		Attr: AttrList{
			VCS:                 true,
			RevisionShort:       false,
			Branch:              true,
			Tag:                 true,
			State:               true, // TODO
			HasRemote:           true,
			CommitsAheadRemote:  true,
			CommitsBehindRemote: true,
			HasStashedCommits:   true,
			StashedCommits:      true,
			IsDirty:             true,
			HasAddedFiles:       true,
			AddedFiles:          true,
			HasModifiedFiles:    true,
			ModifiedFiles:       true,
			HasDeletedFiles:     true,
			DeletedFiles:        true,
			HasRenamedFiles:     true,
			RenamedFiles:        true,
			HasUnmergedFiles:    true, // TODO
			UnmergedFiles:       true, // TODO
			HasUntrackedFiles:   true,
		},
		DirtyIfUntracked:    true,
		RenamesFromRewrites: true,
		IncludeSubmodules:   true, // TODO
	}
	s.res = Attr{}
	s.want = Attr{
		VCS: VCSGit,
	}
	c.Assert(os.Chdir(c.MkDir()), IsNil)
	git("init")
	gitconfig()
}

func (s *GitSuite) TearDownSuite(c *C) {
	c.Assert(os.Chdir(s.origDir), IsNil)
}

func (s *GitSuite) TestGitNoRepo(c *C) {
	c.Assert(os.Chdir(c.MkDir()), IsNil)
	s.want.VCS = VCSNone
	c.Check(s.VCSInfoGit(), DeepEquals, s.want)
}

func (s *GitSuite) TestGitSeparate(c *C) {
	c.Assert(os.Chdir(c.MkDir()), IsNil)
	repoDir := c.MkDir()
	c.Assert(os.Remove(repoDir), IsNil)
	c.Assert(repoDir, Matches, "^\\S*$") // required to split git params
	git("init --separate-git-dir " + repoDir)
	gitconfig()

	fi, err := os.Stat(".git")
	c.Assert(err, IsNil)
	c.Assert(fi.IsDir(), Equals, false) // repo is in separate dir

	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // no branch

	git("commit --allow-empty -m ROOT")
	s.want.Branch = "master"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // master branch
}

func (s *GitSuite) TestGitRevision(c *C) {
	s.req.Attr.RevisionShort = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // no HEAD: no revision

	git("commit --allow-empty -m ROOT")
	c.Assert(s.VCSInfoGit(), NotNil)
	c.Check(s.res.RevisionShort, Matches, "^[0-9a-f]{7}$") // revision
}

func (s *GitSuite) TestGitBranch(c *C) {
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // default

	git("commit --allow-empty -m ROOT")
	s.want.Branch = "master"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // default

	git("checkout -b fix-a")
	s.want.Branch = "fix-a"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // current (or new?)

	git("branch fix/b master")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // current!

	git("checkout fix/b")
	s.want.Branch = "fix/b"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // switch

	git("commit --allow-empty -m msg1")
	git("checkout @^")
	s.want.Branch = ""
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // detached HEAD

	git("checkout master")
	s.want.Branch = "master"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // attached
}

func (s *GitSuite) TestGitTag(c *C) {
	if testing.Short() {
		c.Skip("slow test")
	}

	s.req.Attr.Branch = false

	git("commit --allow-empty -m ROOT")
	git("commit --allow-empty -m msg1")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // none

	git("tag v1.0.0 -m msg")
	s.want.Tag = "v1.0.0"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // current

	git("commit --allow-empty -m msg2")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // previous

	git("checkout -b fix/a")
	git("commit --allow-empty -m msg3")
	git("tag v1.0.1 -m msg")
	s.want.Tag = "v1.0.1"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // v1.0.1 @ fix/a

	git("checkout -b fix/b")
	git("commit --allow-empty -m msg4")
	git("tag v1.0.2 -m msg")
	s.want.Tag = "v1.0.2"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // v1.0.2 @ fix/a->fix/b

	git("checkout fix/a")
	git("commit --allow-empty -m msg5")
	s.want.Tag = "v1.0.1"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // more work on v1.0.1

	git("checkout fix/b")
	git("commit --allow-empty -m msg6")
	git("commit --allow-empty -m msg7")
	s.want.Tag = "v1.0.2"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // more work on v1.0.2

	git("checkout master")
	git("merge --no-ff fix/b -m msg8")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // latest (or first parent?)

	git("merge --no-ff fix/a -m msg9")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // latest!
}

func (s *GitSuite) TestGitTag_LatestVSName(c *C) {
	if testing.Short() {
		c.Skip("slow test")
	}

	s.req.Attr.Branch = false

	git("commit --allow-empty -m ROOT")
	git("tag alpha -m msg")
	s.want.Tag = "alpha"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // current

	git("tag gamma -m msg")
	s.want.Tag = "gamma"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // latest (or ordered?)

	git("tag beta -m msg")
	s.want.Tag = "beta"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // latest!
}

func (s *GitSuite) TestGitTag_Lightweight(c *C) {
	if testing.Short() {
		c.Skip("slow test")
	}

	s.req.Attr.Branch = false

	git("commit --allow-empty -m ROOT")
	git("tag local")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // lightweight

	git("tag global -m msg")
	s.want.Tag = "global"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // annotated
}

func (s *GitSuite) TestGitRemote(c *C) {
	s.req.Attr.Branch = false

	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // empty repo has no remote

	originDir, err := os.Getwd()
	c.Assert(err, IsNil)
	cloneDir := c.MkDir()
	c.Assert(os.Remove(cloneDir), IsNil)
	c.Assert(originDir, Matches, "^\\S*$")         // required to split git params
	c.Assert(cloneDir, Matches, "^\\S*$")          // required to split git params
	git("config receive.denyCurrentBranch ignore") // allow push from clone
	git("commit --allow-empty -m ROOT")            // needs not empty repo for clone
	git("clone " + originDir + " " + cloneDir)
	c.Assert(os.Chdir(cloneDir), IsNil)
	gitconfig()

	s.want.HasRemote = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone has remote

	git("commit --allow-empty -m msg1")
	s.want.HasRemote = true
	s.want.CommitsAheadRemote = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone go forward

	git("push")
	s.want.CommitsAheadRemote = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone push

	c.Assert(os.Chdir(originDir), IsNil)
	git("reset --hard") // needed after push to non-bare repo
	git("commit --allow-empty -m msg2")
	git("commit --allow-empty -m msg3")
	c.Assert(os.Chdir(cloneDir), IsNil)
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone unaware

	git("fetch")
	s.want.CommitsBehindRemote = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone behind

	git("commit --allow-empty -m msg4")
	git("commit --allow-empty -m msg5")
	git("commit --allow-empty -m msg6")
	s.want.CommitsAheadRemote = 3
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone diverse

	git("checkout @^")
	s.want.HasRemote = false
	s.want.CommitsAheadRemote = 0
	s.want.CommitsBehindRemote = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // detached HEAD

	git("checkout master")
	git("merge origin/master")
	s.want.HasRemote = true
	s.want.CommitsAheadRemote = 4
	s.want.CommitsBehindRemote = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone updated

	git("push")
	s.want.CommitsAheadRemote = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone in sync

	git("remote add upstream " + originDir)
	git("fetch upstream")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone has two remotes

	git("checkout -b feature")
	s.want.HasRemote = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // new branch has no remote

	git("checkout master")
	git("remote rm origin")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // remote removed

	git("checkout feature")
	git("branch -u upstream/master")
	s.want.HasRemote = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // remote added

	c.Assert(os.Chdir(originDir), IsNil)
	s.want.HasRemote = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // origin has no remote
}

func (s *GitSuite) TestGitStash(c *C) {
	s.req.Attr.Branch = false
	s.req.Attr.IsDirty = false
	s.req.Attr.HasAddedFiles = false
	s.req.Attr.HasModifiedFiles = false
	s.req.Attr.HasDeletedFiles = false
	s.req.Attr.HasRenamedFiles = false
	s.req.Attr.HasUnmergedFiles = false
	s.req.Attr.HasUntrackedFiles = false
	git("commit --allow-empty -m ROOT") // can't stash in empty repo

	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	git("stash --include-untracked")
	s.want.HasStashedCommits = true
	s.want.StashedCommits = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // stashed

	c.Assert(ioutil.WriteFile("b.txt", nil, 0666), IsNil)
	git("stash --include-untracked")
	s.want.StashedCommits = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // more stashed

	git("stash pop")
	s.want.StashedCommits = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // less stashed

	git("stash pop")
	s.want.HasStashedCommits = false
	s.want.StashedCommits = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // nothing stashed
}

func (s *GitSuite) TestGitHasUntrackedFiles(c *C) {
	s.enable_possible_optimizations()
	s.req.Attr.IsDirty = false
	s.req.Attr.HasUntrackedFiles = true

	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	s.want.HasUntrackedFiles = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked

	git("add -- a.txt")
	s.want.HasUntrackedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // no untracked

	git("reset -- a.txt")
	s.req.Attr.HasAddedFiles = true
	s.want.HasUntrackedFiles = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked when HasAdded
}

func (s *GitSuite) TestGitHasUntrackedFiles_HEAD(c *C) {
	git("commit --allow-empty -m ROOT")
	s.want.Branch = "master"
	s.TestGitHasUntrackedFiles(c)
}

func (s *GitSuite) TestGitIsDirty_IfUntracked(c *C) {
	s.req.Attr.HasAddedFiles = false
	s.req.Attr.AddedFiles = false

	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	s.want.IsDirty = true
	s.want.HasUntrackedFiles = s.req.Attr.HasUntrackedFiles
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked is dirty

	s.req.DirtyIfUntracked = false
	s.want.IsDirty = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked is not dirty
	s.req.DirtyIfUntracked = true

	c.Assert(os.Remove("a.txt"), IsNil)
	s.want.HasUntrackedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // no untracked is not dirty

	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	c.Assert(ioutil.WriteFile("b.txt", nil, 0666), IsNil)
	git("add -- a.txt")
	s.want.IsDirty = true
	s.want.HasUntrackedFiles = s.req.Attr.HasUntrackedFiles
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added is dirty

	s.req.DirtyIfUntracked = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added is always dirty

	git("reset -- a.txt")
	s.want.IsDirty = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked is not dirty
}

func (s *GitSuite) TestGitIsDirty_IfUntracked2(c *C) {
	s.req.Attr.HasUntrackedFiles = !s.req.Attr.HasUntrackedFiles
	s.TestGitIsDirty_IfUntracked(c)
}

func (s *GitSuite) TestGitIsDirty_IfUntracked_HEAD(c *C) {
	git("commit --allow-empty -m ROOT")
	s.want.Branch = "master"
	s.TestGitIsDirty_IfUntracked(c)
}

func (s *GitSuite) TestGitIsDirty_IfUntracked_HEAD2(c *C) {
	git("commit --allow-empty -m ROOT")
	s.want.Branch = "master"
	s.TestGitIsDirty_IfUntracked2(c)
}

func (s *GitSuite) TestGitAddedFiles(c *C) {
	s.enable_possible_optimizations()
	s.req.Attr.HasAddedFiles = true
	s.req.Attr.AddedFiles = true
	s.want.IsDirty = s.req.Attr.IsDirty

	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	git("add -- a.txt")
	s.want.HasAddedFiles = true
	s.want.AddedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added one

	c.Assert(ioutil.WriteFile("b.txt", nil, 0666), IsNil)
	git("add -- b.txt")
	s.want.AddedFiles = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added two

	s.req.Attr.HasAddedFiles = false
	s.want.HasAddedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without Has

	s.req.Attr.HasAddedFiles = true
	s.req.Attr.AddedFiles = false
	s.want.HasAddedFiles = true
	s.want.AddedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without amount

	s.req.Attr.HasAddedFiles = false
	s.want.HasAddedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without Has&amount
}

func (s *GitSuite) TestGitAddedFiles2(c *C) {
	s.req.Attr.IsDirty = !s.req.Attr.IsDirty
	s.TestGitAddedFiles(c)
}

func (s *GitSuite) TestGitAddedFiles_HEAD(c *C) {
	git("commit --allow-empty -m ROOT")
	s.want.Branch = "master"
	s.TestGitAddedFiles(c)
}

func (s *GitSuite) TestGitAddedFiles_HEAD2(c *C) {
	git("commit --allow-empty -m ROOT")
	s.want.Branch = "master"
	s.TestGitAddedFiles2(c)
}

func (s *GitSuite) TestGitModifiedFiles(c *C) {
	s.enable_possible_optimizations()
	s.req.Attr.HasModifiedFiles = true
	s.req.Attr.ModifiedFiles = true
	s.want.IsDirty = s.req.Attr.IsDirty

	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	c.Assert(ioutil.WriteFile("b.txt", nil, 0666), IsNil)
	c.Assert(ioutil.WriteFile("c.txt", nil, 0666), IsNil)
	git("add -- a.txt b.txt c.txt")
	git("commit -m ROOT")
	s.want.Branch = "master"

	c.Assert(ioutil.WriteFile("a.txt", []byte("a"), 0666), IsNil)
	s.want.HasModifiedFiles = true
	s.want.ModifiedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // modified a (content)

	c.Assert(os.Chmod("b.txt", 0755), IsNil)
	s.want.ModifiedFiles = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // modified b (perm)

	c.Assert(os.Remove("c.txt"), IsNil)
	c.Assert(os.Symlink("a.txt", "c.txt"), IsNil)
	s.want.ModifiedFiles = 3
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // modified c (type)

	git("add -- a.txt")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // modified (index+workdir)

	git("add .")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // modified (index)

	s.req.Attr.HasModifiedFiles = false
	s.want.HasModifiedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without Has

	s.req.Attr.HasModifiedFiles = true
	s.req.Attr.ModifiedFiles = false
	s.want.HasModifiedFiles = true
	s.want.ModifiedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without amount

	s.req.Attr.HasModifiedFiles = false
	s.want.HasModifiedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without Has&amount
}

func (s *GitSuite) TestGitModifiedFiles2(c *C) {
	s.req.Attr.IsDirty = !s.req.Attr.IsDirty
	s.TestGitModifiedFiles(c)
}

func (s *GitSuite) TestGitDeletedFiles(c *C) {
	s.enable_possible_optimizations()
	s.req.Attr.HasDeletedFiles = true
	s.req.Attr.DeletedFiles = true
	s.want.IsDirty = s.req.Attr.IsDirty

	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	c.Assert(ioutil.WriteFile("b.txt", nil, 0666), IsNil)
	git("add -- a.txt b.txt")
	git("commit -m ROOT")
	s.want.Branch = "master"

	c.Assert(os.Remove("a.txt"), IsNil)
	s.want.HasDeletedFiles = true
	s.want.DeletedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // deleted one (workdir)

	c.Assert(os.Remove("b.txt"), IsNil)
	git("add -- b.txt")
	s.want.DeletedFiles = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // deleted two (index+workdir)

	git("add .")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // deleted two (index)

	s.req.Attr.HasDeletedFiles = false
	s.want.HasDeletedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without Has

	s.req.Attr.HasDeletedFiles = true
	s.req.Attr.DeletedFiles = false
	s.want.HasDeletedFiles = true
	s.want.DeletedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without amount

	s.req.Attr.HasDeletedFiles = false
	s.want.HasDeletedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without Has&amount
}

func (s *GitSuite) TestGitDeletedFiles2(c *C) {
	s.req.Attr.IsDirty = !s.req.Attr.IsDirty
	s.TestGitDeletedFiles(c)
}

const rewriteBlock = 80 // `git status` use 64

func (s *GitSuite) TestGitRenamedFiles(c *C) {
	s.enable_possible_optimizations()
	s.req.Attr.HasRenamedFiles = true
	s.req.Attr.RenamedFiles = true
	s.want.IsDirty = s.req.Attr.IsDirty

	c.Assert(ioutil.WriteFile("a.txt", bytes.Repeat([]byte{'a'}, rewriteBlock), 0666), IsNil)
	c.Assert(ioutil.WriteFile("b.txt", bytes.Repeat([]byte{'b'}, rewriteBlock), 0666), IsNil)
	c.Assert(ioutil.WriteFile("c.txt", bytes.Repeat([]byte{'c'}, rewriteBlock), 0666), IsNil)
	git("add -- a.txt b.txt c.txt")
	git("commit -m ROOT")
	s.want.Branch = "master"

	c.Assert(os.Rename("a.txt", "a2.txt"), IsNil)
	git("add .")
	s.want.HasRenamedFiles = true
	s.want.RenamedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // rename a

	c.Assert(os.Remove("b.txt"), IsNil)
	c.Assert(ioutil.WriteFile("b2.txt", bytes.Repeat([]byte{'b'}, rewriteBlock+1), 0666), IsNil)
	git("add -- b.txt b2.txt")
	s.want.RenamedFiles = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // rename b (minor)

	c.Assert(os.Remove("c.txt"), IsNil)
	c.Assert(ioutil.WriteFile("c2.txt", bytes.Repeat([]byte{'c'}, rewriteBlock-1), 0666), IsNil)
	git("add -- c.txt c2.txt")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // rename c (major)

	s.req.Attr.HasRenamedFiles = false
	s.want.HasRenamedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without Has

	s.req.Attr.HasRenamedFiles = true
	s.req.Attr.RenamedFiles = false
	s.want.HasRenamedFiles = true
	s.want.RenamedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without amount

	s.req.Attr.HasRenamedFiles = false
	s.want.HasRenamedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // without Has&amount
}

func (s *GitSuite) TestGitRenamedFiles2(c *C) {
	s.req.Attr.IsDirty = !s.req.Attr.IsDirty
	s.TestGitRenamedFiles(c)
}

func (s *GitSuite) TestGitRenamesFromRewrites(c *C) {
	c.ExpectFailure("https://github.com/libgit2/git2go/issues/380")
	s.enable_possible_optimizations()
	s.req.Attr.HasRenamedFiles = true
	s.req.Attr.RenamedFiles = true
	s.want.IsDirty = s.req.Attr.IsDirty

	c.Assert(ioutil.WriteFile("a.txt", bytes.Repeat([]byte{'a'}, rewriteBlock), 0666), IsNil)
	git("add -- a.txt")
	git("commit -m ROOT")
	s.want.Branch = "master"

	c.Assert(os.Remove("a.txt"), IsNil)
	c.Assert(ioutil.WriteFile("a2.txt", bytes.Repeat([]byte{'a'}, rewriteBlock+1), 0666), IsNil)
	git("add -- a.txt a2.txt")
	s.want.HasRenamedFiles = true
	s.want.RenamedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // RenamesFromRewrites

	s.req.RenamesFromRewrites = false
	s.want.HasRenamedFiles = false
	s.want.RenamedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // no RenamesFromRewrites
}

func (s *GitSuite) TestGitRenamesFromRewrites2(c *C) {
	s.req.Attr.IsDirty = !s.req.Attr.IsDirty
	s.TestGitRenamesFromRewrites(c)
}

func (s *GitSuite) TestGitDirtyFiles(c *C) {
	c.Assert(ioutil.WriteFile("added.txt", []byte("new"), 0666), IsNil)
	c.Assert(ioutil.WriteFile("modified.txt", []byte("original"), 0666), IsNil)
	c.Assert(ioutil.WriteFile("deleted.txt", []byte("junk"), 0666), IsNil)
	c.Assert(ioutil.WriteFile("renamed.txt", bytes.Repeat([]byte{'a'}, 64), 0666), IsNil)
	c.Assert(ioutil.WriteFile("untracked.txt", []byte("useless"), 0666), IsNil)
	git("add -- modified.txt deleted.txt renamed.txt")
	git("commit --allow-empty -m ROOT")
	s.want.Branch = "master"

	git("add -- added.txt")
	c.Assert(ioutil.WriteFile("modified.txt", []byte("modified"), 0666), IsNil)
	c.Assert(os.Remove("deleted.txt"), IsNil)
	c.Assert(os.Rename("renamed.txt", "renamed2.txt"), IsNil)
	git("add -- renamed.txt renamed2.txt")
	s.want.IsDirty = true
	s.want.HasAddedFiles = true
	s.want.AddedFiles = 1
	s.want.HasModifiedFiles = true
	s.want.ModifiedFiles = 1
	s.want.HasDeletedFiles = true
	s.want.DeletedFiles = 1
	s.want.HasRenamedFiles = true
	s.want.RenamedFiles = 1
	s.want.HasUntrackedFiles = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want)

	git("add .")
	s.want.AddedFiles = 2
	s.want.HasUntrackedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want)

	git("commit -m msg1")
	s.want.IsDirty = false
	s.want.HasAddedFiles = false
	s.want.AddedFiles = 0
	s.want.HasModifiedFiles = false
	s.want.ModifiedFiles = 0
	s.want.HasDeletedFiles = false
	s.want.DeletedFiles = 0
	s.want.HasRenamedFiles = false
	s.want.RenamedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want)
}

// States: TODO
// - merge interrupted:
//   * checkout -b fix; commit; checkout master: none
//   * merge --no-ff fix -m '': merge
//   * merge --abort: none
//   * merge --no-ff fix -m '': merge
//   * commit -m msg: none
// - merge conflict:
//   * TODO UnmergedFiles
