package output

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

func ConfigureCLIColorProfile(w io.Writer) termenv.Profile {
	profile := detectColorProfile(w)
	lipgloss.SetColorProfile(profile)
	return profile
}

func detectColorProfile(w io.Writer) termenv.Profile {
	if disableColorOutput() {
		return termenv.Ascii
	}
	if forceColorOutput() {
		return termenv.EnvColorProfile()
	}
	if file, ok := w.(*os.File); ok {
		if term.IsTerminal(int(file.Fd())) {
			return termenv.NewOutput(w).ColorProfile()
		}
		return termenv.Ascii
	}
	return termenv.EnvColorProfile()
}

func disableColorOutput() bool {
	if termenv.EnvNoColor() {
		return true
	}
	if val, ok := lookupEnv("CLICOLOR"); ok && strings.TrimSpace(val) == "0" {
		return true
	}
	if val, ok := lookupEnv("TERM"); ok && strings.EqualFold(strings.TrimSpace(val), "dumb") {
		return true
	}
	return false
}

func forceColorOutput() bool {
	if val, ok := lookupEnv("CLICOLOR_FORCE"); ok && envTruthy(val) {
		return true
	}
	if val, ok := lookupEnv("FORCE_COLOR"); ok && envTruthy(val) {
		return true
	}
	return false
}

func envTruthy(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func lookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func detectOutputWidth(w io.Writer) int {
	file, ok := w.(*os.File)
	if !ok {
		return 0
	}
	fd := int(file.Fd())
	if !term.IsTerminal(fd) {
		return 0
	}
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return 0
	}
	return width
}
