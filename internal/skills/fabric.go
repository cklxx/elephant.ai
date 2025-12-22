package skills

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	fabricEnvVar              = "ALEX_FABRIC_DIR"
	fabricDisableRemoteEnvVar = "ALEX_FABRIC_DISABLE_REMOTE"
	fabricRepoZipURL          = "https://codeload.github.com/danielmiessler/Fabric/zip/refs/heads/main"
)

func loadFabricLibrary(ctx context.Context) (Library, error) {
	root, fromEnv, err := fabricRootFromEnv()
	if err != nil {
		return Library{}, err
	}

	if root == "" {
		if isFabricRemoteDisabled() {
			return Library{}, nil
		}
		root, err = ensureFabricRepo(ctx)
		if err != nil {
			return Library{}, nil
		}
	}

	library, err := loadFabricFromRoot(root)
	if err != nil {
		if fromEnv {
			return Library{}, err
		}
		return Library{}, nil
	}

	return library, nil
}

func loadFabricFromRoot(root string) (Library, error) {
	patternsDir := filepath.Join(root, "data", "patterns")
	entries, err := os.ReadDir(patternsDir)
	if err != nil {
		return Library{}, fmt.Errorf("read fabric patterns: %w", err)
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		systemPath := filepath.Join(patternsDir, name, "system.md")
		systemContent, err := os.ReadFile(systemPath)
		if err != nil {
			return Library{}, fmt.Errorf("read fabric system prompt %s: %w", systemPath, err)
		}

		userPath := filepath.Join(patternsDir, name, "user.md")
		userContent, _ := os.ReadFile(userPath)

		systemText := normalizeNewlines(string(systemContent))
		userText := normalizeNewlines(string(userContent))

		skill := Skill{
			Name:        NormalizeName(name),
			Title:       fabricTitleFromName(name),
			Description: fabricDescription(systemText),
			Body:        fabricSkillBody(systemText, userText),
			SourcePath:  systemPath,
		}
		skills = append(skills, skill)
	}

	return buildLibrary(skills, root)
}

func ensureFabricRepo(ctx context.Context) (string, error) {
	cacheDir := fabricCacheDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create fabric cache dir: %w", err)
	}

	extracted := filepath.Join(cacheDir, "Fabric-main")
	if _, err := os.Stat(extracted); err == nil {
		return extracted, nil
	}

	zipPath := filepath.Join(cacheDir, "fabric.zip")
	if err := downloadFile(ctx, fabricRepoZipURL, zipPath); err != nil {
		return "", err
	}

	if err := unzipFile(zipPath, cacheDir); err != nil {
		return "", err
	}

	return extracted, nil
}

func fabricRootFromEnv() (string, bool, error) {
	raw, ok := os.LookupEnv(fabricEnvVar)
	if !ok {
		return locateLocalFabric(), false, nil
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", true, nil
	}

	resolved := filepath.Clean(trimmed)
	info, err := os.Stat(resolved)
	if err != nil {
		return "", true, fmt.Errorf("stat fabric dir: %w", err)
	}
	if !info.IsDir() {
		return "", true, fmt.Errorf("fabric dir %s is not a directory", resolved)
	}

	return resolved, true, nil
}

func locateLocalFabric() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	candidates := []string{
		filepath.Join(home, "fabric"),
		filepath.Join(home, "Fabric"),
		filepath.Join(home, ".fabric"),
	}

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

func isFabricRemoteDisabled() bool {
	value, ok := os.LookupEnv(fabricDisableRemoteEnvVar)
	if !ok {
		return false
	}
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "1" || value == "true" || value == "yes"
}

func fabricCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return filepath.Join(os.TempDir(), "alex", "fabric")
	}
	return filepath.Join(base, "alex", "fabric")
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download fabric repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download fabric repo: unexpected status %d", resp.StatusCode)
	}

	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create fabric zip: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write fabric zip: %w", err)
	}
	return nil
}

func unzipFile(path, dest string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	// Normalize destination directory and use its absolute path for safety checks.
	destAbs, err := filepath.Abs(filepath.Clean(dest))
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}

	for _, f := range r.File {
		// Clean the entry name to eliminate redundant separators and dots.
		name := filepath.Clean(f.Name)
		// Reject empty names.
		if name == "." || name == "" {
			continue
		}
		// Reject absolute paths from the archive.
		if filepath.IsAbs(name) {
			return fmt.Errorf("zip entry %q has absolute path", f.Name)
		}
		// On Windows, also reject paths starting with a drive letter like "C:".
		if vol := filepath.VolumeName(name); vol != "" {
			return fmt.Errorf("zip entry %q has invalid volume %q", f.Name, vol)
		}

		target := filepath.Join(destAbs, name)
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("resolve target path for %q: %w", f.Name, err)
		}

		// Ensure the target path is within the destination directory (Zip Slip protection).
		destPrefix := destAbs + string(os.PathSeparator)
		targetPrefix := targetAbs + string(os.PathSeparator)
		if !strings.HasPrefix(targetPrefix, destPrefix) {
			return fmt.Errorf("zip entry %q would be extracted outside destination", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetAbs, f.Mode()); err != nil {
				return fmt.Errorf("create dir %s: %w", targetAbs, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return fmt.Errorf("prepare dir %s: %w", targetAbs, err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		file, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("create file %s: %w", targetAbs, err)
		}

		if _, err := io.Copy(file, rc); err != nil {
			rc.Close()
			file.Close()
			return fmt.Errorf("write file %s: %w", targetAbs, err)
		}

		rc.Close()
		file.Close()
	}

	return nil
}

func fabricDescription(system string) string {
	for _, line := range strings.Split(system, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, ".", 2)
		candidate := strings.TrimSpace(parts[0])
		if candidate != "" {
			return candidate
		}
	}
	return "Fabric pattern"
}

func fabricSkillBody(system, user string) string {
	system = strings.TrimSpace(system)
	user = strings.TrimSpace(user)

	if user == "" {
		return system
	}

	var builder strings.Builder
	builder.WriteString(system)
	builder.WriteString("\n\n---\n## User Template\n")
	builder.WriteString(user)
	return strings.TrimSpace(builder.String())
}

func fabricTitleFromName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' })
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	if len(parts) == 0 {
		return name
	}
	return strings.Join(parts, " ")
}

func normalizeNewlines(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}
