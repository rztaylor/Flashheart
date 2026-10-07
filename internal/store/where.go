package store

import (
	"time"

	"github.com/rztaylor/flashheart/internal/gitinfo"
)

// cwdCache is the cwd cache, relative to the root (agent-protocol §2).
const cwdCache = ".flashheart/cache/cwd.json"

// Where finds a working directory's repository, branch and worktree through
// the root's cwd cache, writing the cache back only when it changed. A
// cache that cannot be written is not an error: the next caller resolves
// again.
func (s *Store) Where(cwd string, now time.Time) gitinfo.Info {
	return s.where(cwd, now, true)
}

// Locate is Where without writing the cache back, for callers that only
// read (agent-protocol §7.1).
func (s *Store) Locate(cwd string, now time.Time) gitinfo.Info {
	return s.where(cwd, now, false)
}

func (s *Store) where(cwd string, now time.Time, write bool) gitinfo.Info {
	data, _ := s.ReadFile(cwdCache)
	cache := gitinfo.ParseCache(data)
	info, _ := cache.Resolve(cwd, now)
	if write && cache.Changed() {
		_ = s.WriteFileAtomic(cwdCache, cache.Marshal())
	}
	return info
}
