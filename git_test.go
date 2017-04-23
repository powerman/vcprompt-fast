package main

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	. "gopkg.in/check.v1"
)

func git(args string) {
	err := exec.Command("git", strings.Split(args, " ")...).Run()
	if err != nil {
		panic(err)
	}
}

type GitSuite struct {
	origDir string
	ctx     context.Context
	wanted  VCSInfoWanted
	opt     Options
	res     *VCSInfo
	want    *VCSInfo
}

var _ = Suite(&GitSuite{})

func (s *GitSuite) VCSInfoGit() *VCSInfo {
	s.res = VCSInfoGit(s.ctx, s.wanted, s.opt)
	s.res.Fix(s.wanted, s.opt)
	return s.res
}

func (s *GitSuite) SetUpSuite(c *C) {
	s.ctx = context.Background()
	var err error
	s.origDir, err = os.Getwd()
	c.Assert(err, IsNil)
}

func (s *GitSuite) SetUpTest(c *C) {
	// These defaults make it easier to compare full VCSInfo.
	s.opt = Options{
		DirtyIfUntracked: true,
	}
	s.wanted = VCSInfoWanted{
		Revision:              false,
		Branch:                true,
		Tag:                   true,
		Action:                true, // TODO
		HasRemote:             true,
		RemoteDivergedCommits: true,
		HasStashedCommits:     true,
		StashedCommits:        true,
		HasUntrackedFiles:     true,
		IsDirty:               true,
		DirtyFiles:            true,
	}
	s.res = nil
	s.want = &VCSInfo{
		VCS:    VCSGit,
		Branch: "master",
	}
	c.Assert(os.Chdir(c.MkDir()), IsNil)
	git("init")
	git("config user.email root@localhost")
	git("config user.name Nobody")
}

func (s *GitSuite) TearDownTest(c *C) {}

func (s *GitSuite) TearDownSuite(c *C) {
	c.Assert(os.Chdir(s.origDir), IsNil)
}

func (s *GitSuite) TestGitNoRepo(c *C) {
	c.Assert(os.Chdir(c.MkDir()), IsNil)
	s.want.Branch = ""
	c.Check(s.VCSInfoGit(), IsNil)
}

func (s *GitSuite) TestGitEmptyRepo(c *C) {
	s.wanted.Revision = true
	s.want.Branch = ""
	c.Check(s.VCSInfoGit(), DeepEquals, s.want)
}

func (s *GitSuite) TestGitRevision(c *C) {
	git("commit --allow-empty -m msg")
	s.wanted.Revision = true
	c.Assert(s.VCSInfoGit(), NotNil)
	c.Check(s.res.RevisionShort, Matches, "^[0-9a-f]{7}$")
}

func (s *GitSuite) TestGitBranch(c *C) {
	git("commit --allow-empty -m msg")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // default
	git("checkout -b fix-a")
	s.want.Branch = "fix-a"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // current (or new?)
	git("branch fix/b master")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // current!
	git("checkout fix/b")
	s.want.Branch = "fix/b"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // switch
	git("commit --allow-empty -m msg")
	git("checkout @^")
	s.want.Branch = ""
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // detached HEAD
	git("checkout master")
	s.want.Branch = "master"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // attached
	s.wanted.Branch = false
	s.want.Branch = ""
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disabled
}

func (s *GitSuite) TestGitTag(c *C) {
	s.wanted.Branch = false
	s.want.Branch = ""
	git("commit --allow-empty -m msg")
	git("commit --allow-empty -m msg")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // none
	git("tag v1.0.0")
	s.want.Tag = "v1.0.0"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // current
	git("commit --allow-empty -m msg")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // previous
	git("tag alpha")
	s.want.Tag = "alpha"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // current (with previous)
	git("tag gamma")
	s.want.Tag = "gamma"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // latest (or ordered?)
	git("tag beta")
	s.want.Tag = "beta"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // latest!
	git("checkout -b fix/a")
	git("commit --allow-empty -m msg")
	git("tag v1.0.1")
	s.want.Tag = "v1.0.1"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // v1.0.1
	git("checkout -b fix/b")
	git("commit --allow-empty -m msg")
	git("tag v1.0.2")
	s.want.Tag = "v1.0.2"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // v1.0.2
	git("checkout fix/a")
	git("commit --allow-empty -m msg")
	s.want.Tag = "v1.0.1"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // more work on v1.0.1
	git("checkout fix/b")
	git("commit --allow-empty -m msg")
	git("commit --allow-empty -m msg")
	s.want.Tag = "v1.0.2"
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // more work on v1.0.2
	git("checkout master")
	git("merge --no-ff fix/b -m msg")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // latest (or parent?)
	git("merge --no-ff fix/a -m msg")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // latest!
	s.wanted.Tag = false
	s.want.Tag = ""
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disabled
}

func (s *GitSuite) TestGitRemote(c *C) {
	originDir, err := os.Getwd()
	c.Assert(err, IsNil)
	cloneDir := c.MkDir()
	c.Assert(os.Remove(cloneDir), IsNil)
	c.Assert(originDir, Matches, "^\\S*$") // required to split git params
	c.Assert(cloneDir, Matches, "^\\S*$")  // required to split git params
	git("clone " + originDir + " " + cloneDir)

	c.Assert(os.Chdir(cloneDir), IsNil)
	s.want.HasRemote = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone has remote
	c.Assert(os.Chdir(originDir), IsNil)
	s.want.HasRemote = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // origin has no remote
	c.Assert(os.Chdir(cloneDir), IsNil)
	git("commit --allow-empty -m msg")
	s.want.HasRemote = true
	s.want.RemoteAheadCommits = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone go forward
	git("push")
	s.want.RemoteAheadCommits = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone push
	c.Assert(os.Chdir(originDir), IsNil)
	git("commit --allow-empty -m msg")
	git("commit --allow-empty -m msg")
	c.Assert(os.Chdir(cloneDir), IsNil)
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone unaware
	git("fetch")
	s.want.RemoteBehindCommits = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone behind
	git("commit --allow-empty -m msg")
	git("commit --allow-empty -m msg")
	git("commit --allow-empty -m msg")
	s.want.RemoteAheadCommits = 3
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone diverse
	s.wanted.RemoteDivergedCommits = false
	s.want.RemoteAheadCommits = 0
	s.want.RemoteBehindCommits = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disable diverged
	s.wanted.RemoteDivergedCommits = true
	s.want.RemoteAheadCommits = 3
	git("merge origin master")
	s.want.RemoteBehindCommits = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone updated
	git("push")
	s.want.RemoteAheadCommits = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone in sync
	git("remote add fake http://localhost/")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone has remotes
	s.wanted.HasRemote = false
	s.want.HasRemote = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disabled
	s.wanted.HasRemote = true
	git("remote rm origin")
	s.want.HasRemote = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone has remote
	git("remote rm fake")
	s.want.HasRemote = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // clone has no remote
}

func (s *GitSuite) TestGitStash(c *C) {
	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	git("stash -u")
	s.want.HasStashedCommits = true
	s.want.StashedCommits = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // stashed
	c.Assert(ioutil.WriteFile("b.txt", nil, 0666), IsNil)
	git("stash -u")
	s.want.StashedCommits = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // more stashed
	s.wanted.HasStashedCommits = false
	s.want.HasStashedCommits = false
	s.want.StashedCommits = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disabled
	s.wanted.HasStashedCommits = true
	s.wanted.StashedCommits = false
	s.want.HasStashedCommits = true
	s.want.StashedCommits = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // count disabled
	s.wanted.StashedCommits = true
	git("stash pop")
	s.want.StashedCommits = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // less stashed
	git("stash pop")
	s.want.StashedCommits = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // nothing stashed
}

func (s *GitSuite) TestGitHasUntrackedFiles(c *C) {
	s.wanted.IsDirty = false
	s.wanted.DirtyFiles = false
	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	s.want.HasUntrackedFiles = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked
	s.wanted.HasUntrackedFiles = false
	s.want.HasUntrackedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disabled
	s.wanted.HasUntrackedFiles = true
	git("add -- a.txt")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // no untracked
}

func (s *GitSuite) TestGitIsDirty_IfUntracked(c *C) {
	s.wanted.HasUntrackedFiles = false
	s.wanted.DirtyFiles = false
	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	c.Assert(ioutil.WriteFile("b.txt", nil, 0666), IsNil)
	s.opt.DirtyIfUntracked = false
	s.want.IsDirty = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked is not dirty
	s.opt.DirtyIfUntracked = true
	s.want.IsDirty = true
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked is dirty
	git("add -- a.txt")
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added is dirty
	s.opt.DirtyIfUntracked = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added is always dirty
	git("reset -- a.txt")
	s.want.IsDirty = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // untracked is not dirty
}

func (s *GitSuite) TestGitIsDirty_IfUntracked_Unwanted(c *C) {
	s.wanted.HasUntrackedFiles = false
	s.TestGitIsDirty_IfUntracked(c)
}

// UnmergedFiles will be tested with Action, not here.
func (s *GitSuite) TestGitDirtyFiles(c *C) {
	s.wanted.HasUntrackedFiles = false
	s.want.IsDirty = true

	c.Assert(ioutil.WriteFile("a.txt", nil, 0666), IsNil)
	c.Assert(ioutil.WriteFile("b.txt", nil, 0666), IsNil)
	git("add -- a.txt")
	s.want.HasAddedFiles = true
	s.want.AddedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added

	git("add -- b.txt")
	s.want.AddedFiles = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added two

	c.Assert(ioutil.WriteFile("a.txt", []byte("adata"), 0666), IsNil)
	s.want.HasModifiedFiles = true
	s.want.ModifiedFiles = 1
	s.want.AddedFiles = 1                       // probably it also may be 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added and modified

	git("commit -m msg")
	s.want.HasAddedFiles = false
	s.want.AddedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // modified

	c.Assert(ioutil.WriteFile("b.txt", []byte("bdata"), 0666), IsNil)
	s.want.ModifiedFiles = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // modified two

	c.Assert(os.Remove("a.txt"), IsNil)
	s.want.HasDeletedFiles = true
	s.want.DeletedFiles = 1
	s.want.ModifiedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // modified and deleted

	c.Assert(os.Remove("b.txt"), IsNil)
	s.want.DeletedFiles = 2
	s.want.HasModifiedFiles = false
	s.want.ModifiedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // deleted two
	git("reset --hard")
	s.want.HasDeletedFiles = false
	s.want.DeletedFiles = 0

	git("mv -- a.txt a2.txt")
	s.want.HasRenamedFiles = true
	s.want.RenamedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // renamed
	git("mv -- b.txt b2.txt")
	s.want.RenamedFiles = 2
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // renamed two

	c.Assert(os.Remove("b2.txt"), IsNil)
	s.want.HasDeletedFiles = true
	s.want.DeletedFiles = 1
	s.want.RenamedFiles = 1
	c.Assert(ioutil.WriteFile("c.txt", nil, 0666), IsNil)
	c.Assert(ioutil.WriteFile("d.txt", nil, 0666), IsNil)
	git("add -- c.txt d.txt")
	s.want.HasAddedFiles = true
	s.want.AddedFiles = 2
	c.Assert(ioutil.WriteFile("c.txt", []byte("cdata"), 0666), IsNil)
	s.want.HasModifiedFiles = true
	s.want.ModifiedFiles = 1
	s.want.AddedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // added, modified, deleted, renamed

	s.wanted.DirtyFiles = false
	s.want.HasAddedFiles = true
	s.want.HasModifiedFiles = true
	s.want.HasDeletedFiles = true
	s.want.HasRenamedFiles = true
	s.want.AddedFiles = 0
	s.want.ModifiedFiles = 0
	s.want.DeletedFiles = 0
	s.want.RenamedFiles = 0
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disabled DirtyFiles
	s.wanted.IsDirty = false
	s.want.IsDirty = false
	s.want.HasAddedFiles = false
	s.want.HasModifiedFiles = false
	s.want.HasDeletedFiles = false
	s.want.HasRenamedFiles = false
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disabled IsDirty & DirtyFiles
	s.wanted.DirtyFiles = true
	s.want.HasAddedFiles = true
	s.want.HasModifiedFiles = true
	s.want.HasDeletedFiles = true
	s.want.HasRenamedFiles = true
	s.want.AddedFiles = 1
	s.want.ModifiedFiles = 1
	s.want.DeletedFiles = 1
	s.want.RenamedFiles = 1
	c.Check(s.VCSInfoGit(), DeepEquals, s.want) // disabled IsDirty
}

// Actions: TODO (check supported by vcs-info)
// - merge interrupted:
//   * checkout -b fix; commit; checkout master: none
//   * merge --no-ff fix -m '': merge
//   * merge --abort: none
//   * merge --no-ff fix -m '': merge
//   * commit -m msg: none
// - merge conflict:
//   * TODO UnmergedFiles
// - rebase interrupted:
//   * TODO
// - rebase conflict:
//   * TODO
