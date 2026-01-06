package agent_eval

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGeneralAgentDataset(t *testing.T) {
	datasetPath := filepath.Join("datasets", "general_agent_eval.json")

	instances, err := loadGeneralAgentDataset(datasetPath, 5)
	if err != nil {
		t.Fatalf("load general agent dataset: %v", err)
	}

	if len(instances) != 5 {
		t.Fatalf("expected 5 instances with limit applied, got %d", len(instances))
	}

	first := instances[0]
	if first.ProblemStatement == "" {
		t.Fatal("expected problem statement to be populated")
	}

	if first.Hints == "" || !strings.Contains(first.Hints, "-") {
		t.Fatalf("expected formatted constraints in hints, got %q", first.Hints)
	}

	if domain, ok := first.Metadata["domain"].(string); !ok || domain == "" {
		t.Fatalf("expected domain metadata to be present, got %v", first.Metadata["domain"])
	}

	if surface, ok := first.Metadata["surface"].(string); !ok || surface == "" {
		t.Fatalf("expected surface metadata to default to web, got %v", first.Metadata["surface"])
	}

	all, err := loadGeneralAgentDataset(datasetPath, 50)
	if err != nil {
		t.Fatalf("load full general agent dataset: %v", err)
	}
	if len(all) < len(instances) {
		t.Fatalf("expected full dataset to be at least as large as limited slice, got %d", len(all))
	}

	t.Run("EmbeddedDataset", func(t *testing.T) {
		embedded, err := loadGeneralAgentDataset("", 2)
		if err != nil {
			t.Fatalf("load embedded general agent dataset: %v", err)
		}
		if len(embedded) != 2 {
			t.Fatalf("expected limit to apply on embedded dataset, got %d", len(embedded))
		}
		if embedded[0].Metadata["surface"] != "web" {
			t.Fatalf("expected embedded dataset surface to be web, got %v", embedded[0].Metadata["surface"])
		}
	})
}
