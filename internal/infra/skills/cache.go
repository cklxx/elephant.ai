package skills

import (
	"strings"
	"sync"
	"time"
)

var (
	cacheMu       sync.Mutex
	cachedLibrary *Library
	cacheTime     time.Time
	cacheRoot     string
)

// CachedLibrary returns a cached skill library if within TTL.
func CachedLibrary(ttl time.Duration) (Library, error) {
	if ttl <= 0 {
		return DefaultLibrary()
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	root := LocateDefaultDir()
	if cachedLibrary != nil && time.Since(cacheTime) < ttl && strings.EqualFold(cacheRoot, root) {
		return *cachedLibrary, nil
	}

	lib, err := Load(root)
	if err != nil {
		return Library{}, err
	}
	cachedLibrary = &lib
	cacheTime = time.Now()
	cacheRoot = root
	return lib, nil
}

// InvalidateCache clears the cached library.
func InvalidateCache() {
	cacheMu.Lock()
	cachedLibrary = nil
	cacheRoot = ""
	cacheMu.Unlock()
}
