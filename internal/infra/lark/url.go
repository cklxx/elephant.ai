package lark

import (
	"net/url"
	"strings"
)

// apiToWebDomain maps a Lark API base domain to the corresponding user-facing
// web domain. Known mappings are used directly; unknown domains have the "open."
// prefix stripped as a best-effort fallback.
func apiToWebDomain(baseDomain string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseDomain))
	if err != nil || parsed.Host == "" {
		return baseDomain
	}
	host := parsed.Host

	known := map[string]string{
		"open.feishu.cn":      "feishu.cn",
		"open.larksuite.com":  "larksuite.com",
		"open.larkoffice.com": "larkoffice.com",
	}
	if webHost, ok := known[host]; ok {
		return parsed.Scheme + "://" + webHost
	}
	// Fallback: strip "open." prefix.
	if strings.HasPrefix(host, "open.") {
		return parsed.Scheme + "://" + strings.TrimPrefix(host, "open.")
	}
	return parsed.Scheme + "://" + host
}

// BuildDocumentURL constructs the user-facing URL for a Lark document.
func BuildDocumentURL(baseDomain, documentID string) string {
	if baseDomain == "" || documentID == "" {
		return ""
	}
	return apiToWebDomain(baseDomain) + "/docx/" + documentID
}

// BuildWikiNodeURL constructs the user-facing URL for a Lark wiki node.
func BuildWikiNodeURL(baseDomain, nodeToken string) string {
	if baseDomain == "" || nodeToken == "" {
		return ""
	}
	return apiToWebDomain(baseDomain) + "/wiki/" + nodeToken
}

// BuildSpreadsheetURL constructs the user-facing URL for a Lark spreadsheet.
func BuildSpreadsheetURL(baseDomain, spreadsheetToken string) string {
	if baseDomain == "" || spreadsheetToken == "" {
		return ""
	}
	return apiToWebDomain(baseDomain) + "/sheets/" + spreadsheetToken
}
