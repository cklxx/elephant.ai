package diagram

import "strings"

type cardThemeSpec struct {
	Background     string
	Border         string
	Shadow         string
	IconBackground string
	IconBorder     string
}

type textThemeSpec struct {
	Title string
	Body  string
	Muted string
}

func normalizeTheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "dark":
		return "dark"
	default:
		return "light"
	}
}

func backgroundTheme(theme string) string {
	if theme == "dark" {
		return "linear-gradient(135deg, #0b1220, #111827)"
	}
	return "linear-gradient(135deg, #f8fafc, #e2e8f0)"
}

func cardTheme(theme string) cardThemeSpec {
	if theme == "dark" {
		return cardThemeSpec{
			Background:     "#0f172a",
			Border:         "#1e293b",
			Shadow:         "0 10px 30px rgba(0,0,0,.35)",
			IconBackground: "rgba(148,163,184,.12)",
			IconBorder:     "#334155",
		}
	}
	return cardThemeSpec{
		Background:     "#ffffff",
		Border:         "#e2e8f0",
		Shadow:         "0 10px 30px rgba(2,6,23,.12)",
		IconBackground: "rgba(165,180,252,.25)",
		IconBorder:     "#cbd5e1",
	}
}

func textTheme(theme string) textThemeSpec {
	if theme == "dark" {
		return textThemeSpec{
			Title: "#e2e8f0",
			Body:  "#cbd5e1",
			Muted: "#94a3b8",
		}
	}
	return textThemeSpec{
		Title: "#0f172a",
		Body:  "#334155",
		Muted: "#475569",
	}
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

