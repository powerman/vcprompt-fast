package main

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
//     ** if use %% avoid conflicts with zsh %F/%K/%B/%f/%k/%b if possible
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
// - show something special when timeout happens to avoid needs in
//   configuring anything per-repo
func main() {
	// TODO
	// - parse flags
	//   * TODO check all vcprompt/vcs_info/etc. implementations and
	//     try to define most-compatible to everyone flags list
	//   * output help
	//   * setup log to STDERR only in debug mode, otherwise to /dev/null
	// - detect VCS here, to avoid trying different VCS engines
	//   one-by-one and have each one read same dirs again and again
	//   * TODO check how vcs_info does this
	//   * chdir to repo root, to avoid more detection by executed
	//     commands and simplify rest of code
	// - exit without output (except log) if no VCS detected
	// - get rest of configuration (from environment?)
	// - call VCS engine to gather facts according to configuration
	// - output facts in user-defined format
	//   * boolean fact  detected     ? pre/user-defined value : nothing
	//   * boolean fact  not detected ? pre/user-defined value : nothing
	//   * enum fact                    pre/user-defined value
	//   * value fact    not empty    ? fact's value           : nothing
	//   * value fact    empty        ? pre/user-defined value : nothing
	//   * is boolean fact == true    ? sub-format strings for true/false
	//   * is enum fact == some value ? sub-format strings for true/false
	// - add boolean facts for: revision, branch, tag
	// - different format strings (choose one either by VCSType or by
	//   repo path - let user define pairs "repo path - format name"):
	//   * default
	//   * for each VCSType
	//   * custom names
	//   * predefined-hardcoded (for quick start / compatibility modes)
	// - different option lists just like formats - per repo/VCSType/…
	// - probably it makes sense to use same format strings as zsh:
	//   * we anyway should support %b for branch because of vcprompt
	//   * this force %… in format, with %% for % and conflict with
	//     zsh's PS1 %b (just like this happens for vcs_info)
	//   * conditional %(… true false) and %) for ) is no worse than
	//     anything else (except if I like to use Go templates here)
	//   * TODO check all vcprompt/vcs_info/etc. implementations and
	//     try to define most-compatible to everyone format syntax
}
