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
	data, _ := s.ReadFile(cwdCache)
	cache := gitinfo.ParseCache(data)
	info, _ := cache.Resolve(cwd, now)
	if cache.Changed() {
		_ = s.WriteFileAtomic(cwdCache, cache.Marshal())
	}
	return info
}
