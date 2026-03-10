package memory

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (i *Indexer) watchLoop() {
	for {
		select {
		case <-i.stopCh:
			return
		case event, ok := <-i.watcher.Events:
			if !ok {
				return
			}
			i.handleEvent(event)
		case err, ok := <-i.watcher.Errors:
			if !ok {
				return
			}
			i.logger.Warn("Memory index watcher error: %v", err)
		}
	}
}

func (i *Indexer) handleEvent(event fsnotify.Event) {
	if event.Name == "" {
		return
	}
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}
	if event.Op&fsnotify.Create != 0 {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			i.handleDirCreate(event.Name)
			return
		}
	}
	if !isMemoryFile(event.Name) {
		return
	}
	i.scheduleIndex(event.Name)
}

func (i *Indexer) handleDirCreate(path string) {
	if err := i.addWatchDir(path); err != nil {
		return
	}
	if isUserDir(i.rootDir, path) {
		_ = i.addWatchDir(filepath.Join(path, dailyDirName))
	}
}

func (i *Indexer) scheduleIndex(path string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if timer, ok := i.timers[path]; ok {
		timer.Stop()
	}
	i.timers[path] = time.AfterFunc(defaultIndexDebounce, func() {
		_ = i.indexPath(context.Background(), path)
	})
}

func (i *Indexer) addWatchRoots() error {
	if err := i.addWatchDir(i.rootDir); err != nil {
		return err
	}
	_ = i.addWatchDir(filepath.Join(i.rootDir, dailyDirName))
	legacyUsersDir := filepath.Join(i.rootDir, legacyUserDirName)
	_ = i.addWatchDir(legacyUsersDir)

	entries, err := os.ReadDir(i.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == legacyUserDirName {
			legacyEntries, err := os.ReadDir(filepath.Join(i.rootDir, legacyUserDirName))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			for _, legacyEntry := range legacyEntries {
				if !legacyEntry.IsDir() {
					continue
				}
				userPath := filepath.Join(legacyUsersDir, legacyEntry.Name())
				_ = i.addWatchDir(userPath)
				_ = i.addWatchDir(filepath.Join(userPath, dailyDirName))
			}
			continue
		}
		if isReservedUserDirName(name) {
			continue
		}
		userPath := filepath.Join(i.rootDir, name)
		_ = i.addWatchDir(userPath)
		_ = i.addWatchDir(filepath.Join(userPath, dailyDirName))
	}
	return nil
}

func (i *Indexer) addWatchDir(path string) error {
	if i == nil || i.watcher == nil {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return i.watcher.Add(path)
}
