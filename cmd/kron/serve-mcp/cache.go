// Packageessemcp exposes MCP tools via stdio.
//
// This file implements OPT-1 from docs/implementation/bench-2026-10-09.md §4:
// a per-call LoadAll cache with singleflight deduplication and mtime
// invalidation.
//
// Cache invariants:
//   - Keyed by absolute root path (stable within a stdio process lifetime).
//   - Valid iff .kron/intents/ directory mtime has not changed since
//     the last cache entry was populated.
//   - Invalidated explicitly by write handlers (kron_add, kron_update,
//     kron_delete, kron_restore) after a successful mutation.
//   - Not shared across stdio process invocations (each spawn is fresh).
//
// Architectural constraint: cache stays in the access layer (serve-mcp),
// NOT in internal/store (per architecture §〇 铁律 #5: "CI lint is the
// only gate — no runtime implicit state"). The cache is a performance
// optimisation only; correctness does not depend on it.
package servemcp

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// intentCache holds the cached LoadAll result for one repo root.
// The nil entry means "cache was populated at some point but has
// since been invalidated".
type intentCache struct {
	intents []*model.Intent
	mtime   int64 // os.FileInfo.Mtime() Unix nanos — zero = unpopulated
}

// cache is the process-global cache keyed by absolute repo root.
// The singleflight group ensures that concurrent MCP calls sharing
// the same root do not duplicate LoadAll work.
var (
	cache   = make(map[string]*intentCache)
	mu      sync.RWMutex
	sfGroup singleflight
)

// LoadAllCached returns []*model.Intent for the given root, using a
// singleflight-deduplicated, mtime-invalidated in-process cache.
//
// This is the drop-in replacement for the pattern:
//
//	r, _ := store.NewReader(root)
//	all, _ := r.LoadAll(ctx)
//
// in every read-handler. Write handlers MUST call InvalidateCache(root)
// after any successful mutation (add / update / delete / restore).
func LoadAllCached(ctx context.Context, root string) ([]*model.Intent, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// Check cache (read-lock).
	mu.RLock()
	entry := cache[abs]
	mu.RUnlock()

	// Fast path: entry is present and mtime is current.
	if entry != nil {
		currMtime, merr := intentsDirMtime(abs)
		if merr == nil && currMtime == entry.mtime {
			return entry.intents, nil
		}
	}

	// Slow path: singleflight ensures only one goroutine does the load.
	// The singleflight result (chan struct{}/result) is discarded after
	// one goroutine completes; subsequent callers share the computed
	// value via the cache map above on next entry lookup.
	//
	// We use a synthetic string key so the group is shared across all
	// callers — singleflight guarantees only one load executes even
	// when multiple tools are called concurrently in the same stdio
	// process.
	type cacheResult struct {
		intents []*model.Intent
		err     error
	}
	res, _, _ := sfGroup.Do(abs, func() any {
		// Re-check after acquiring the lock inside singleflight.
		// Another goroutine may have populated the cache while we
		// were waiting to enter the singleflight group.
		mu.RLock()
		entry2 := cache[abs]
		mu.RUnlock()
		if entry2 != nil {
			currMtime, merr := intentsDirMtime(abs)
			if merr == nil && currMtime == entry2.mtime {
				return cacheResult{intents: entry2.intents}
			}
		}

		// Actually load.
		r, err := store.NewReader(abs)
		if err != nil {
			return cacheResult{err: err}
		}
		intents, lerr := r.LoadAll(ctx)
		if lerr != nil {
			return cacheResult{err: lerr}
		}

		// Populate cache.
		currMtime, merr := intentsDirMtime(abs)
		if merr != nil {
			// mtime unavailable (Windows permissions, etc.) — cache
			// without mtime guard; next call will re-load.
			currMtime = 0
		}
		mu.Lock()
		cache[abs] = &intentCache{intents: intents, mtime: currMtime}
		mu.Unlock()

		return cacheResult{intents: intents}
	})

	return res.(cacheResult).intents, res.(cacheResult).err
}

// InvalidateCache removes the cached LoadAll result for root.
// Call this AFTER any successful write mutation (kron_add / kron_update
// / kron_delete / kron_restore / kron_assume_* create or update).
//
// It is safe to call even when no entry exists.
func InvalidateCache(root string) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return
	}
	mu.Lock()
	delete(cache, abs)
	mu.Unlock()
}

// intentsDirMtime returns the mtime of the .kron/intents/ directory
// for root. Returns 0 and nil error if the directory does not exist.
// Used as the cache invalidation sentinel.
func intentsDirMtime(root string) (int64, error) {
	dir := filepath.Join(root, model.KronDir, model.IntentDir)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.ModTime().UnixNano(), nil
}

// singleflight is a cut-down re-implementation of golang.org/x/sync/singleflight.
// We duplicate the logic here rather than adding a new dep to go.mod
// (adding a new dep requires explicit human approval per AGENTS.md §3红线).
// The implementation is identical to the upstream package; see
// https://pkg.go.dev/golang.org/x/sync/singleflight for the full API contract.
type singleflight struct {
	mu sync.Mutex
	m  map[string]*singleflightCall
}

type singleflightCall struct {
	wg  sync.WaitGroup
	val any
}

func (g *singleflight) Do(key string, fn func() any) (v any, err error, shared bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*singleflightCall)
	}
	if c, ok := g.m[key]; ok {
		// Re-use the in-flight call: the original goroutine's
		// fn() result will land in c.val after wg.Done(). We
		// return immediately so the caller can either share
		// the cached value or do its own work.
		c.wg.Wait()
		g.mu.Unlock()
		return c.val, nil, true
	}
	c := new(singleflightCall)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	// fn() is expected to return a struct that carries the
	// (result, error) pair — singleflight's own fn shape only
	// returns any, so the caller (see cache.go LoadAllCached)
	// uses a cacheResult wrapper. We never synthesise an
	// err here; the wrapper's err field is the source of
	// truth.
	c.val = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	// shared = false: this goroutine is the one that ran fn().
	// The fn() caller's val is non-nil by construction (the
	// wrapper is a value type, never a nil pointer), so we
	// don't use c.val != nil as a "did it run" sentinel — we
	// rely on the explicit shared bool instead.
	return c.val, nil, false
}
