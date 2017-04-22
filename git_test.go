package main

import (
	"context"
	"os"
	"os/exec"

	. "gopkg.in/check.v1"
)

type GitSuite struct {
	ctx     context.Context
	origDir string
}

var _ = Suite(&GitSuite{})

func (s *GitSuite) SetUpSuite(c *C) {
	s.ctx = context.Background()
	var err error
	s.origDir, err = os.Getwd()
	if err != nil {
		c.Fatal(err)
	}
	err = os.Chdir(c.MkDir())
	if err != nil {
		c.Fatal(err)
	}
}

func (s *GitSuite) SetUpTest(c *C) {}

func (s *GitSuite) TearDownTest(c *C) {}

func (s *GitSuite) TearDownSuite(c *C) {
	err := os.Chdir(s.origDir)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *GitSuite) TestGit(c *C) {
	var opt Options
	var wanted VCSInfoWanted
	var want VCSInfo
	c.Check(VCSInfoGit(s.ctx, wanted, opt), IsNil) // no repo
	c.Check(exec.Command("git", "init").Run(), IsNil)
	want = VCSInfo{VCS: VCSGit}
	c.Check(VCSInfoGit(s.ctx, wanted, opt), DeepEquals, &want)
}
