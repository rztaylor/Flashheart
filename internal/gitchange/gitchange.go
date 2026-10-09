package gitchange

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// maxPaths bounds how many changed files are looked at.
const maxPaths = 2000

// ChangedSince reports whether worktree's work changed after since: a file
// git lists as changed or untracked (not ignored) was modified after it, or
// a commit made after it changed a file that was. A deleted file counts by
// its directory's modification time. When counts is set, a modification
// time after since counts only if counts accepts it (the caller's shell
// command windows); committed files are judged by their modification time
// the same way. gitDir is the worktree's own git directory, whose HEAD
// reflog says cheaply whether any commit can be newer. It fails when git is
// missing, the directory is not a worktree or ctx ends.
func ChangedSince(ctx context.Context, worktree, gitDir string, since time.Time, counts func(modified time.Time) bool) (bool, error) {
	counted := func(modified time.Time) bool {
		return modified.After(since) && (counts == nil || counts(modified))
	}
	status, err := git(ctx, worktree, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames")
	if err != nil {
		return false, err
	}
	var paths []string
	for _, entry := range bytes.Split(status, []byte{0}) {
		// "XY path": two status letters and a space.
		if len(entry) > 3 {
			paths = append(paths, string(entry[3:]))
		}
	}
	if modified(worktree, paths, counted) {
		return true, nil
	}
	if !headMovedAfter(gitDir, since) {
		return false, nil
	}
	committed, err := git(ctx, worktree, "log", "-z", "--no-renames", "--name-only", "--format=", "--since="+since.UTC().Format(time.RFC3339), "HEAD")
	if err != nil {
		return false, err
	}
	paths = paths[:0]
	for _, name := range bytes.Split(committed, []byte{0}) {
		if name = bytes.TrimSpace(name); len(name) > 0 {
			paths = append(paths, string(name))
		}
	}
	return modified(worktree, paths, counted), nil
}

func git(ctx context.Context, worktree string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-C", worktree}, args...)...)
	cmd.WaitDelay = 10 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}

// headMovedAfter reports whether the worktree's HEAD reflog changed after
// since. Without a reflog no commit is looked for.
func headMovedAfter(gitDir string, since time.Time) bool {
	if gitDir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(gitDir, "logs", "HEAD"))
	return err == nil && info.ModTime().After(since)
}

// modified reports whether any of paths has a modification time counted
// accepts.
func modified(worktree string, paths []string, counted func(time.Time) bool) bool {
	for _, path := range paths[:min(len(paths), maxPaths)] {
		name := filepath.Join(worktree, filepath.FromSlash(path))
		info, err := os.Lstat(name)
		// Deleted: removing it touched the nearest directory still there.
		for dir := name; err != nil && dir != worktree && dir != filepath.Dir(dir); {
			dir = filepath.Dir(dir)
			info, err = os.Lstat(dir)
		}
		if err == nil && counted(info.ModTime()) {
			return true
		}
	}
	return false
}
