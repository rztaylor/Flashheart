// Package gitinfo finds a working directory's repository without running
// git: the main checkout (which names the project, PRJ-2), the worktree
// containing the directory and its branch, read from the HEAD file. A small
// cache keyed by directory and validated by HEAD's modification time keeps
// most hooks from walking the file system (agent-protocol §2).
//
// It reads only git metadata under the given directory and its parents. The
// cache's bytes are stored by its caller through store; turning a repository
// into a board project (collisions, creation) belongs to store and hooks.
package gitinfo
