package app

import "testing"

func TestParseTaskAnalysisStructuredXML(t *testing.T) {
	content := `<task_analysis>
  <action>Implementing feature</action>
  <goal>Add dark mode toggle to settings page</goal>
  <approach>Audit UI components then update theme wiring</approach>
  <success_criteria>
    <criterion>Toggle persists across reloads</criterion>
    <criterion>Update documentation</criterion>
  </success_criteria>
  <task_breakdown>
    <step requires_external_research="false" requires_retrieval="true">
      <description>Inspect current theme provider</description>
      <reason>Need to review internal theme helpers</reason>
    </step>
    <step requires_external_research="true" requires_retrieval="false">
      <description>Validate accessibility contrast</description>
      <reason>Requires latest WCAG guidance</reason>
    </step>
  </task_breakdown>
  <retrieval_plan should_retrieve="true">
    <local_queries>
      <query>theme provider</query>
      <query>dark mode config</query>
    </local_queries>
    <search_queries>
      <query>WCAG contrast ratios</query>
    </search_queries>
    <crawl_urls>
      <url>https://www.w3.org/WAI/standards-guidelines/wcag/</url>
    </crawl_urls>
    <knowledge_gaps>
      <gap>Current contrast thresholds</gap>
    </knowledge_gaps>
    <notes>Use existing design tokens where possible</notes>
  </retrieval_plan>
</task_analysis>`

	analysis := parseTaskAnalysis(content)
	if analysis == nil {
		t.Fatal("expected analysis to be parsed")
	}
	if analysis.ActionName != "Implementing feature" {
		t.Fatalf("unexpected action: %q", analysis.ActionName)
	}
	if len(analysis.Criteria) != 2 {
		t.Fatalf("expected success criteria, got %#v", analysis.Criteria)
	}
	if len(analysis.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %#v", analysis.Steps)
	}
	if !analysis.Steps[0].NeedsExternalContext || !analysis.Steps[1].NeedsExternalContext {
		t.Fatalf("expected both steps to mark context needs, got %#v", analysis.Steps)
	}
	if !analysis.Retrieval.ShouldRetrieve {
		t.Fatal("expected retrieval flag to be true")
	}
	if len(analysis.Retrieval.LocalQueries) != 2 || analysis.Retrieval.LocalQueries[0] != "theme provider" {
		t.Fatalf("expected local queries preserved, got %#v", analysis.Retrieval.LocalQueries)
	}
	if len(analysis.Retrieval.SearchQueries) != 1 || analysis.Retrieval.SearchQueries[0] != "WCAG contrast ratios" {
		t.Fatalf("expected search queries preserved, got %#v", analysis.Retrieval.SearchQueries)
	}
	if len(analysis.Retrieval.CrawlURLs) != 1 {
		t.Fatalf("expected crawl url present, got %#v", analysis.Retrieval.CrawlURLs)
	}
	if analysis.Retrieval.Notes != "Use existing design tokens where possible" {
		t.Fatalf("unexpected retrieval notes: %q", analysis.Retrieval.Notes)
	}
}

func TestParseTaskAnalysisStructuredXMLWithFences(t *testing.T) {
	content := "Here is the structured plan you asked for:\n" +
		"```xml\n" +
		"<task_analysis>\n" +
		"  <action>Drafting summary</action>\n" +
		"  <goal>Produce an executive overview</goal>\n" +
		"  <approach>Review gathered intel then synthesize</approach>\n" +
		"  <success_criteria>\n" +
		"    <criterion>Reference at least two sources</criterion>\n" +
		"  </success_criteria>\n" +
		"  <task_breakdown>\n" +
		"    <step requires_retrieval=\"true\">\n" +
		"      <description>Review internal notes</description>\n" +
		"      <reason>Need freshest inputs</reason>\n" +
		"    </step>\n" +
		"  </task_breakdown>\n" +
		"  <retrieval_plan should_retrieve=\"false\">\n" +
		"    <local_queries>\n" +
		"      <query>executive notes</query>\n" +
		"    </local_queries>\n" +
		"  </retrieval_plan>\n" +
		"</task_analysis>\n" +
		"```\n" +
		"Let me know if you need more details."

	analysis := parseTaskAnalysis(content)
	if analysis == nil {
		t.Fatal("expected analysis to be parsed from fenced XML")
	}
	if analysis.ActionName != "Drafting summary" {
		t.Fatalf("unexpected action: %q", analysis.ActionName)
	}
	if !analysis.Retrieval.ShouldRetrieve {
		t.Fatal("expected retrieval flag to be coerced true when queries present")
	}
	if len(analysis.Retrieval.LocalQueries) != 1 || analysis.Retrieval.LocalQueries[0] != "executive notes" {
		t.Fatalf("expected local queries preserved, got %#v", analysis.Retrieval.LocalQueries)
	}
	if len(analysis.Steps) != 1 || !analysis.Steps[0].NeedsExternalContext {
		t.Fatalf("expected step to mark external context, got %#v", analysis.Steps)
	}
}

func TestExtractTaskAnalysisFragment(t *testing.T) {
	raw := "``````\n" +
		"\n" +
		"Some intro\n" +
		"<task_analysis should_ignore=\"yes\">\n" +
		"  <action>Plan</action>\n" +
		"</task_analysis>\n" +
		"```"
	fragment := extractTaskAnalysisFragment(raw)
	expected := "<task_analysis should_ignore=\"yes\">\n" +
		"  <action>Plan</action>\n" +
		"</task_analysis>"
	if fragment != expected {
		t.Fatalf("expected fragment %q, got %q", expected, fragment)
	}

	if extractTaskAnalysisFragment("no xml here") != "" {
		t.Fatal("expected empty fragment when no XML present")
	}
}
