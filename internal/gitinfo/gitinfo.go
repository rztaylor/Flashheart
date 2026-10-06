package gitinfo

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Info describes where a working directory sits. Repo and Project are empty
// outside git (PRJ-4).
type Info struct {
	// Repo is the main checkout's path; worktrees share it.
	Repo string `json:"repo,omitempty"`
	// Project is the main checkout's directory name, before any collision
	// rule or sanitising.
	Project string `json:"project,omitempty"`
	// Worktree is the top of the working tree containing the directory.
	Worktree string `json:"worktree,omitempty"`
	// Branch is the checked-out branch, or the short SHA of a detached HEAD.
	Branch string `json:"branch,omitempty"`
	// Head is the HEAD file the branch was read from.
	Head string `json:"head,omitempty"`
}

// maxGitFile bounds reads of .git, HEAD and commondir files.
const maxGitFile = 4096

func readSmall(name string) (string, bool) {
	file, err := os.Open(name)
	if err != nil {
		return "", false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxGitFile))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

// Resolve finds cwd's repository by walking up to the nearest .git. It never
// fails: anything unreadable is treated as outside git.
func Resolve(cwd string) Info {
	if cwd == "" || !filepath.IsAbs(cwd) {
		return Info{}
	}
	// Real paths, so a checkout reached through a symlink matches the real
	// paths git records for its worktrees.
	dir, err := filepath.EvalSymlinks(filepath.Clean(cwd))
	if err != nil {
		return Info{}
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return Info{}
	}
	for {
		dotgit := filepath.Join(dir, ".git")
		if info, err := os.Lstat(dotgit); err == nil {
			if resolved, ok := fromDotGit(dir, dotgit, info); ok {
				return resolved
			}
			return Info{}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Info{}
		}
		dir = parent
	}
}

func fromDotGit(worktree, dotgit string, info os.FileInfo) (Info, bool) {
	gitdir := dotgit
	if !info.IsDir() {
		content, ok := readSmall(dotgit)
		target, found := strings.CutPrefix(content, "gitdir:")
		if !ok || !found {
			return Info{}, false
		}
		target = strings.TrimSpace(target)
		if !filepath.IsAbs(target) {
			target = filepath.Join(worktree, target)
		}
		gitdir = filepath.Clean(target)
	}
	if real, err := filepath.EvalSymlinks(gitdir); err == nil {
		gitdir = real
	}
	common := gitdir
	if relative, ok := readSmall(filepath.Join(gitdir, "commondir")); ok && relative != "" {
		if !filepath.IsAbs(relative) {
			relative = filepath.Join(gitdir, relative)
		}
		common = filepath.Clean(relative)
		if real, err := filepath.EvalSymlinks(common); err == nil {
			common = real
		}
	}
	// PRJ-2: the main checkout is the common git directory's parent. A bare
	// repository (repo.git) names itself; anything else, such as a
	// submodule's directory under .git/modules, is its own checkout.
	repo := worktree
	switch base := filepath.Base(common); {
	case base == ".git":
		repo = filepath.Dir(common)
	case strings.HasSuffix(base, ".git") && !strings.Contains(filepath.ToSlash(common), "/.git/"):
		repo = strings.TrimSuffix(common, ".git")
	}
	head := filepath.Join(gitdir, "HEAD")
	return Info{Repo: repo, Project: filepath.Base(repo), Worktree: worktree, Branch: branch(head), Head: head}, true
}

func branch(head string) string {
	content, ok := readSmall(head)
	if !ok {
		return ""
	}
	if ref, found := strings.CutPrefix(content, "ref:"); found {
		ref = strings.TrimSpace(ref)
		if name, isBranch := strings.CutPrefix(ref, "refs/heads/"); isBranch {
			return name
		}
		return strings.TrimPrefix(ref, "refs/")
	}
	if len(content) >= 7 {
		return content[:7]
	}
	return content
}

// Relative returns path relative to the worktree, or "" when it is not
// inside it. Relative paths are taken as relative to the worktree.
func (i Info) Relative(path string) string {
	if i.Worktree == "" || path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(i.Worktree, path)
	}
	path = filepath.Clean(path)
	if rel, ok := inside(i.Worktree, path); ok {
		return rel
	}
	// The path may name the worktree through a symlink: resolve it, or its
	// directory when the file does not exist yet.
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		dir, dirErr := filepath.EvalSymlinks(filepath.Dir(path))
		if dirErr != nil {
			return ""
		}
		real = filepath.Join(dir, filepath.Base(path))
	}
	rel, _ := inside(i.Worktree, real)
	return rel
}

func inside(dir, path string) (string, bool) {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// MaxEntries bounds the cache; the entries resolved longest ago are dropped
// (a cache hit does not count, so hits never cause a write).
const MaxEntries = 500

// Entry is one cached resolution with its validator.
type Entry struct {
	Info
	// Stamp is the validator: HEAD's modification time inside git, the
	// directory's outside it, in Unix nanoseconds.
	Stamp int64 `json:"stamp"`
	Used  int64 `json:"used"`
}

// Cache maps working directories to resolutions (<root>/.flashheart/cache/cwd.json).
type Cache struct {
	Entries map[string]Entry `json:"entries"`
	changed bool
}

// ParseCache reads the cache file's content; anything unreadable is an
// empty cache.
func ParseCache(data []byte) *Cache {
	cache := &Cache{}
	if len(bytes.TrimSpace(data)) > 0 {
		_ = json.Unmarshal(data, cache)
	}
	if cache.Entries == nil {
		cache.Entries = map[string]Entry{}
	}
	return cache
}

// Changed reports whether Resolve added or replaced an entry since parsing.
func (c *Cache) Changed() bool { return c.changed }

func stamp(name string) (int64, bool) {
	info, err := os.Stat(name)
	if err != nil {
		return 0, false
	}
	return info.ModTime().UnixNano(), true
}

// Resolve returns cwd's resolution, from the cache when its validator still
// matches; hit reports a cache hit. A hit does not mark the cache changed,
// so the common case writes nothing.
func (c *Cache) Resolve(cwd string, now time.Time) (Info, bool) {
	if entry, ok := c.Entries[cwd]; ok {
		validator := cwd
		if entry.Head != "" {
			validator = entry.Head
		}
		if current, ok := stamp(validator); ok && current == entry.Stamp {
			return entry.Info, true
		}
	}
	info := Resolve(cwd)
	validator := cwd
	if info.Head != "" {
		validator = info.Head
	}
	if value, ok := stamp(validator); ok {
		c.Entries[cwd] = Entry{Info: info, Stamp: value, Used: now.UnixNano()}
		c.changed = true
	}
	return info, false
}

// Marshal encodes the cache, keeping the MaxEntries most recently used.
func (c *Cache) Marshal() []byte {
	if len(c.Entries) > MaxEntries {
		keys := make([]string, 0, len(c.Entries))
		for key := range c.Entries {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(a, b int) bool { return c.Entries[keys[a]].Used > c.Entries[keys[b]].Used })
		for _, key := range keys[MaxEntries:] {
			delete(c.Entries, key)
		}
	}
	data, _ := json.Marshal(c)
	return data
}
