package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

var (
	red    = color.New(color.FgRed).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	blue   = color.New(color.FgBlue).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
)

// Dependency represents a system dependency
type Dependency struct {
	Name        string
	Command     string
	Description string
	Required    bool
}

// CheckDependencies checks all required system dependencies
func CheckDependencies() error {
	dependencies := []Dependency{
		{
			Name:        "ripgrep",
			Command:     "rg",
			Description: "Fast text search tool used by grep and search functions",
			Required:    true,
		},
		{
			Name:        "ast-grep",
			Command:     "ast-grep",
			Description: "AST-based code search and transformation tool",
			Required:    false,
		},
	}

	var missingDeps []Dependency

	for _, dep := range dependencies {
		if !isDependencyInstalled(dep.Command) {
			missingDeps = append(missingDeps, dep)
		}
	}

	if len(missingDeps) > 0 {
		showMissingDependencies(missingDeps)
		return fmt.Errorf("missing required dependencies: %s", getDependencyNames(missingDeps))
	}

	return nil
}

// isDependencyInstalled checks if a command is available in PATH or can be executed
func isDependencyInstalled(command string) bool {
	// First try exec.LookPath
	_, err := exec.LookPath(command)
	if err == nil {
		return true
	}

	// For ripgrep, test actual functionality instead of version check
	if command == "rg" {
		return testRipgrepFunctionality()
	}

	// For other commands, try version check
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s --version >/dev/null 2>&1", command))
	err = cmd.Run()
	return err == nil
}

// testRipgrepFunctionality tests if ripgrep can actually be used
func testRipgrepFunctionality() bool {
	// Try to run ripgrep on its own help text
	cmd := exec.Command("sh", "-c", "echo 'test' | rg 'test' >/dev/null 2>&1")
	err := cmd.Run()
	return err == nil
}

// showMissingDependencies displays installation instructions for missing dependencies
func showMissingDependencies(deps []Dependency) {
	fmt.Printf("\n%s Missing Required Dependencies\n", red("❌"))
	fmt.Printf("Alex requires the following tools to function properly:\n\n")

	for _, dep := range deps {
		fmt.Printf("%s %s (%s)\n", bold("•"), bold(dep.Name), dep.Command)
		fmt.Printf("  %s\n", dep.Description)
		fmt.Printf("  %s: %s\n\n", "Installation", getInstallationInstructions(dep.Name))
	}

	fmt.Printf("%s Please install the missing dependencies and try again.\n", yellow("💡"))
}

// getInstallationInstructions returns platform-specific installation instructions
func getInstallationInstructions(depName string) string {
	switch depName {
	case "ripgrep":
		return getRipgrepInstallInstructions()
	case "ast-grep":
		return getAstGrepInstallInstructions()
	default:
		return "Please check the official documentation"
	}
}

// getRipgrepInstallInstructions returns ripgrep installation instructions for different platforms
func getRipgrepInstallInstructions() string {
	switch runtime.GOOS {
	case "darwin": // macOS
		return cyan("brew install ripgrep") + " or " + cyan("port install ripgrep")
	case "linux":
		// Detect common Linux distributions
		if isUbuntuOrDebian() {
			return cyan("apt install ripgrep") + " or " + cyan("snap install ripgrep --classic")
		} else if isFedoraOrRHEL() {
			return cyan("dnf install ripgrep") + " or " + cyan("yum install ripgrep")
		} else if isArchLinux() {
			return cyan("pacman -S ripgrep")
		}
		return cyan("Check your package manager") + " or visit " + blue("https://github.com/BurntSushi/ripgrep#installation")
	case "windows":
		return cyan("choco install ripgrep") + " or " + cyan("scoop install ripgrep") + " or download from " + blue("https://github.com/BurntSushi/ripgrep/releases")
	default:
		return "Visit " + blue("https://github.com/BurntSushi/ripgrep#installation")
	}
}

// getAstGrepInstallInstructions returns ast-grep installation instructions for different platforms
func getAstGrepInstallInstructions() string {
	switch runtime.GOOS {
	case "darwin": // macOS
		return cyan("npm install -g @ast-grep/cli") + " or " + cyan("brew install ast-grep")
	case "linux":
		return cyan("npm install -g @ast-grep/cli") + " or " + cyan("pip install ast-grep-cli") + " or visit " + blue("https://ast-grep.github.io/guide/quick-start.html")
	case "windows":
		return cyan("npm install -g @ast-grep/cli") + " or " + cyan("pip install ast-grep-cli") + " or visit " + blue("https://ast-grep.github.io/guide/quick-start.html")
	default:
		return cyan("npm install -g @ast-grep/cli") + " or visit " + blue("https://ast-grep.github.io/guide/quick-start.html")
	}
}

// isUbuntuOrDebian checks if running on Ubuntu or Debian
func isUbuntuOrDebian() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Check for common Debian/Ubuntu indicators
	_, err := exec.LookPath("apt")
	if err == nil {
		return true
	}

	// Check /etc/os-release for additional confirmation
	if output, err := exec.Command("cat", "/etc/os-release").Output(); err == nil {
		content := strings.ToLower(string(output))
		return strings.Contains(content, "ubuntu") || strings.Contains(content, "debian")
	}

	return false
}

// isFedoraOrRHEL checks if running on Fedora or RHEL-based system
func isFedoraOrRHEL() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Check for dnf or yum
	if _, err := exec.LookPath("dnf"); err == nil {
		return true
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return true
	}

	// Check /etc/os-release for additional confirmation
	if output, err := exec.Command("cat", "/etc/os-release").Output(); err == nil {
		content := strings.ToLower(string(output))
		return strings.Contains(content, "fedora") || strings.Contains(content, "rhel") || strings.Contains(content, "centos")
	}

	return false
}

// isArchLinux checks if running on Arch Linux
func isArchLinux() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Check for pacman
	if _, err := exec.LookPath("pacman"); err == nil {
		return true
	}

	// Check /etc/os-release for additional confirmation
	if output, err := exec.Command("cat", "/etc/os-release").Output(); err == nil {
		content := strings.ToLower(string(output))
		return strings.Contains(content, "arch")
	}

	return false
}

// getDependencyNames returns a comma-separated list of dependency names
func getDependencyNames(deps []Dependency) string {
	var names []string
	for _, dep := range deps {
		names = append(names, dep.Name)
	}
	return strings.Join(names, ", ")
}

// CheckDependenciesQuiet checks dependencies without showing detailed messages
// Returns true if all dependencies are available, false otherwise
func CheckDependenciesQuiet() bool {
	return isDependencyInstalled("rg")
}

// CheckDependenciesForTool checks if ripgrep is available when tools need it
func CheckDependenciesForTool() error {
	if !isDependencyInstalled("rg") {
		return fmt.Errorf("ripgrep (rg) is not installed or not accessible in PATH")
	}
	return nil
}
