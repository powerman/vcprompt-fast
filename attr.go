package main

import "log"

// Attr contains values for all VCS attributes.
type Attr struct {
	VCS                 VCSType
	RevisionShort       string
	Branch              string // Hg: bookmark?
	Tag                 string // latest of reachable from current commit
	State               VCSState
	HasRemote           bool
	CommitsAheadRemote  int
	CommitsBehindRemote int
	HasStashedCommits   bool
	StashedCommits      int
	IsDirty             bool
	HasAddedFiles       bool
	AddedFiles          int
	HasModifiedFiles    bool // Git: in index and/or workdir
	ModifiedFiles       int
	HasDeletedFiles     bool
	DeletedFiles        int
	HasRenamedFiles     bool
	RenamedFiles        int
	HasUnmergedFiles    bool
	UnmergedFiles       int
	HasUntrackedFiles   bool // not include ignored files
	// TODO Patch info
}

// AttrList enumerate all VCS attributes which can be detected.
// Some attributes are VCS-specific and ignored by other VCS.
// Some implementations may ignore some attributes because of performance
// issues or because they isn't implemented yet.
type AttrList struct {
	VCS                 bool
	RevisionShort       bool
	Branch              bool
	Tag                 bool
	State               bool
	HasRemote           bool
	CommitsAheadRemote  bool
	CommitsBehindRemote bool
	HasStashedCommits   bool
	StashedCommits      bool
	IsDirty             bool
	HasAddedFiles       bool
	AddedFiles          bool
	HasModifiedFiles    bool
	ModifiedFiles       bool
	HasDeletedFiles     bool
	DeletedFiles        bool
	HasRenamedFiles     bool
	RenamedFiles        bool
	HasUnmergedFiles    bool
	UnmergedFiles       bool
	HasUntrackedFiles   bool
}

// Request describe requested attributes and options which control
// detection of these attributes.
// Some implementations may ignore some options.
type Request struct {
	Attr                AttrList
	DirtyIfUntracked    bool
	RenamesFromRewrites bool
	IncludeSubmodules   bool
}

// Facts contains raw result of repo analysis: which attributes was
// checked, how they was checked (options), detected values.
type Facts struct {
	Req    Request
	Lookup AttrList
	Found  Attr
}

// Result returns requested repo attributes based on detected facts.
func (f Facts) Result() Attr {
	res := f.Found

	res.HasRemote = res.HasRemote ||
		res.CommitsAheadRemote != 0 || res.CommitsBehindRemote != 0
	res.HasStashedCommits = res.HasStashedCommits || res.StashedCommits != 0
	res.HasAddedFiles = res.HasAddedFiles || res.AddedFiles != 0
	res.HasModifiedFiles = res.HasModifiedFiles || res.ModifiedFiles != 0
	res.HasDeletedFiles = res.HasDeletedFiles || res.DeletedFiles != 0
	res.HasRenamedFiles = res.HasRenamedFiles || res.RenamedFiles != 0
	res.HasUnmergedFiles = res.HasUnmergedFiles || res.UnmergedFiles != 0
	res.IsDirty = res.IsDirty ||
		res.HasAddedFiles || res.HasModifiedFiles || res.HasDeletedFiles ||
		res.HasRenamedFiles || res.HasUnmergedFiles ||
		(res.HasUntrackedFiles && f.Req.DirtyIfUntracked)

	return f.Req.Filter(res)
}

// Filter zeroes result attributes which wasn't included in request.
// This makes result more consistent with request, helps to detect some
// bugs earlier and also in tests.
func (req Request) Filter(res Attr) Attr {
	// This can be done using reflect… or just auto-generated.
	// Before using reflect here - make sure it won't slow down.
	// But looks like it's better to auto-generate this, QA and AttrList.
	var z Attr
	if !req.Attr.VCS {
		res.VCS = z.VCS
	}
	if !req.Attr.RevisionShort {
		res.RevisionShort = z.RevisionShort
	}
	if !req.Attr.Branch {
		res.Branch = z.Branch
	}
	if !req.Attr.Tag {
		res.Tag = z.Tag
	}
	if !req.Attr.State {
		res.State = z.State
	}
	if !req.Attr.HasRemote {
		res.HasRemote = z.HasRemote
	}
	if !req.Attr.CommitsAheadRemote {
		res.CommitsAheadRemote = z.CommitsAheadRemote
	}
	if !req.Attr.CommitsBehindRemote {
		res.CommitsBehindRemote = z.CommitsBehindRemote
	}
	if !req.Attr.HasStashedCommits {
		res.HasStashedCommits = z.HasStashedCommits
	}
	if !req.Attr.StashedCommits {
		res.StashedCommits = z.StashedCommits
	}
	if !req.Attr.IsDirty {
		res.IsDirty = z.IsDirty
	}
	if !req.Attr.HasAddedFiles {
		res.HasAddedFiles = z.HasAddedFiles
	}
	if !req.Attr.AddedFiles {
		res.AddedFiles = z.AddedFiles
	}
	if !req.Attr.HasModifiedFiles {
		res.HasModifiedFiles = z.HasModifiedFiles
	}
	if !req.Attr.ModifiedFiles {
		res.ModifiedFiles = z.ModifiedFiles
	}
	if !req.Attr.HasDeletedFiles {
		res.HasDeletedFiles = z.HasDeletedFiles
	}
	if !req.Attr.DeletedFiles {
		res.DeletedFiles = z.DeletedFiles
	}
	if !req.Attr.HasRenamedFiles {
		res.HasRenamedFiles = z.HasRenamedFiles
	}
	if !req.Attr.RenamedFiles {
		res.RenamedFiles = z.RenamedFiles
	}
	if !req.Attr.HasUnmergedFiles {
		res.HasUnmergedFiles = z.HasUnmergedFiles
	}
	if !req.Attr.UnmergedFiles {
		res.UnmergedFiles = z.UnmergedFiles
	}
	if !req.Attr.HasUntrackedFiles {
		res.HasUntrackedFiles = z.HasUntrackedFiles
	}
	return res
}

// QA will log all found attributes which shouldn't have been checked -
// this probably means extra work was done while analysing repo.
func (f Facts) QA() {
	var z Attr
	if !f.Lookup.VCS && f.Found.VCS != z.VCS {
		log.Print("QA notice: redundant VCS")
	}
	if !f.Lookup.RevisionShort && f.Found.RevisionShort != z.RevisionShort {
		log.Print("QA notice: redundant RevisionShort")
	}
	if !f.Lookup.Branch && f.Found.Branch != z.Branch {
		log.Print("QA notice: redundant Branch")
	}
	if !f.Lookup.Tag && f.Found.Tag != z.Tag {
		log.Print("QA notice: redundant Tag")
	}
	if !f.Lookup.State && f.Found.State != z.State {
		log.Print("QA notice: redundant State")
	}
	if !f.Lookup.HasRemote && f.Found.HasRemote != z.HasRemote {
		log.Print("QA notice: redundant HasRemote")
	}
	if !f.Lookup.CommitsAheadRemote && f.Found.CommitsAheadRemote != z.CommitsAheadRemote {
		log.Print("QA notice: redundant CommitsAheadRemote")
	}
	if !f.Lookup.CommitsBehindRemote && f.Found.CommitsBehindRemote != z.CommitsBehindRemote {
		log.Print("QA notice: redundant CommitsBehindRemote")
	}
	if !f.Lookup.HasStashedCommits && f.Found.HasStashedCommits != z.HasStashedCommits {
		log.Print("QA notice: redundant HasStashedCommits")
	}
	if !f.Lookup.StashedCommits && f.Found.StashedCommits != z.StashedCommits {
		log.Print("QA notice: redundant StashedCommits")
	}
	if !f.Lookup.IsDirty && f.Found.IsDirty != z.IsDirty {
		log.Print("QA notice: redundant IsDirty")
	}
	if !f.Lookup.HasAddedFiles && f.Found.HasAddedFiles != z.HasAddedFiles {
		log.Print("QA notice: redundant HasAddedFiles")
	}
	if !f.Lookup.AddedFiles && f.Found.AddedFiles != z.AddedFiles {
		log.Print("QA notice: redundant AddedFiles")
	}
	if !f.Lookup.HasModifiedFiles && f.Found.HasModifiedFiles != z.HasModifiedFiles {
		log.Print("QA notice: redundant HasModifiedFiles")
	}
	if !f.Lookup.ModifiedFiles && f.Found.ModifiedFiles != z.ModifiedFiles {
		log.Print("QA notice: redundant ModifiedFiles")
	}
	if !f.Lookup.HasDeletedFiles && f.Found.HasDeletedFiles != z.HasDeletedFiles {
		log.Print("QA notice: redundant HasDeletedFiles")
	}
	if !f.Lookup.DeletedFiles && f.Found.DeletedFiles != z.DeletedFiles {
		log.Print("QA notice: redundant DeletedFiles")
	}
	if !f.Lookup.HasRenamedFiles && f.Found.HasRenamedFiles != z.HasRenamedFiles {
		log.Print("QA notice: redundant HasRenamedFiles")
	}
	if !f.Lookup.RenamedFiles && f.Found.RenamedFiles != z.RenamedFiles {
		log.Print("QA notice: redundant RenamedFiles")
	}
	if !f.Lookup.HasUnmergedFiles && f.Found.HasUnmergedFiles != z.HasUnmergedFiles {
		log.Print("QA notice: redundant HasUnmergedFiles")
	}
	if !f.Lookup.UnmergedFiles && f.Found.UnmergedFiles != z.UnmergedFiles {
		log.Print("QA notice: redundant UnmergedFiles")
	}
	if !f.Lookup.HasUntrackedFiles && f.Found.HasUntrackedFiles != z.HasUntrackedFiles {
		log.Print("QA notice: redundant HasUntrackedFiles")
	}
}
