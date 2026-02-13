package main

import "testing"

func TestResolveEvalDatasetType(t *testing.T) {
	tests := []struct {
		name        string
		flagValue   string
		datasetPath string
		want        string
	}{
		{
			name:      "explicit dataset type wins",
			flagValue: "general_agent",
			want:      "general_agent",
		},
		{
			name:        "default swe bench path infers swe_bench",
			datasetPath: "evaluation/swe_bench/real_instances.json",
			want:        "swe_bench",
		},
		{
			name:        "swe bench keyword infers swe_bench",
			datasetPath: "tmp/my_swe_bench_cases.json",
			want:        "swe_bench",
		},
		{
			name:        "general agent dataset path infers general_agent",
			datasetPath: "evaluation/agent_eval/datasets/general_agent_eval.json",
			want:        "general_agent",
		},
		{
			name: "empty path defaults general_agent",
			want: "general_agent",
		},
		{
			name:        "non swe json infers file",
			datasetPath: "tmp/custom_cases.json",
			want:        "file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEvalDatasetType(tt.flagValue, tt.datasetPath)
			if got != tt.want {
				t.Fatalf("resolveEvalDatasetType(%q, %q)=%q, want %q", tt.flagValue, tt.datasetPath, got, tt.want)
			}
		})
	}
}
