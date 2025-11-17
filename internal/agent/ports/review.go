package ports

// AutoReviewReport captures the outcome of the automatic reviewer that runs
// after every core agent task. It includes the primary assessment plus any
// follow-up rework attempts that were triggered.
type AutoReviewReport struct {
	Assessment *ResultAssessment `json:"assessment"`
	Rework     *ReworkSummary    `json:"rework,omitempty"`
}

// ResultAssessment represents the reviewer grade for a single task result.
type ResultAssessment struct {
	Score       float64  `json:"score"`
	Grade       string   `json:"grade"`
	Notes       []string `json:"notes,omitempty"`
	NeedsRework bool     `json:"needs_rework"`
}

// ReworkSummary documents how many automated rework attempts were executed
// and whether any of them replaced the final answer.
type ReworkSummary struct {
	Attempted  int      `json:"attempted"`
	Applied    bool     `json:"applied"`
	FinalGrade string   `json:"final_grade,omitempty"`
	FinalScore float64  `json:"final_score,omitempty"`
	Notes      []string `json:"notes,omitempty"`
}
