package search

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxScanTokenSize = 1024 * 1024

func searchTextMatches(root string, re *regexp.Regexp, filter func(path string, entry os.DirEntry) bool, maxResults int) ([]string, int, error) {
	matches := make([]string, 0, 64)
	total := 0

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if filter != nil && !filter(path, entry) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, maxScanTokenSize)

		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				total++
				if maxResults <= 0 || len(matches) < maxResults {
					matches = append(matches, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), lineNum, line))
				}
			}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	})
	return matches, total, err
}

func fileTypeFilter(fileType string) func(path string, entry os.DirEntry) bool {
	trimmed := strings.TrimSpace(fileType)
	if trimmed == "" {
		return nil
	}
	cleaned := strings.TrimPrefix(strings.ToLower(trimmed), ".")
	if cleaned == "" {
		return nil
	}
	return func(path string, entry os.DirEntry) bool {
		if entry.IsDir() {
			return false
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.Name())), ".")
		return ext == cleaned
	}
}
