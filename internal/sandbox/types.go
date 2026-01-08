package sandbox

type Response[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    *T     `json:"data,omitempty"`
}

type BrowserInfo struct {
	UserAgent string          `json:"user_agent"`
	CDPURL    string          `json:"cdp_url"`
	VNCURL    string          `json:"vnc_url"`
	Viewport  BrowserViewport `json:"viewport"`
}

type BrowserViewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type FileReadResult struct {
	Content string `json:"content"`
	File    string `json:"file"`
}

type FileWriteResult struct {
	File         string `json:"file"`
	BytesWritten *int   `json:"bytes_written,omitempty"`
}

type FileInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDirectory  bool   `json:"is_directory"`
	Size         *int64 `json:"size,omitempty"`
	ModifiedTime string `json:"modified_time,omitempty"`
	Permissions  string `json:"permissions,omitempty"`
	Extension    string `json:"extension,omitempty"`
}

type FileListResult struct {
	Path           string     `json:"path"`
	Files          []FileInfo `json:"files"`
	TotalCount     int        `json:"total_count"`
	DirectoryCount int        `json:"directory_count"`
	FileCount      int        `json:"file_count"`
}

type FileSearchResult struct {
	File        string   `json:"file"`
	Matches     []string `json:"matches"`
	LineNumbers []int    `json:"line_numbers"`
}

type FileReplaceResult struct {
	File          string `json:"file"`
	ReplacedCount int    `json:"replaced_count"`
}

type ShellCommandResult struct {
	SessionID string          `json:"session_id"`
	Command   string          `json:"command"`
	Status    string          `json:"status"`
	Output    *string         `json:"output,omitempty"`
	Console   []ConsoleRecord `json:"console,omitempty"`
	ExitCode  *int            `json:"exit_code,omitempty"`
}

type ConsoleRecord struct {
	PS1     string `json:"ps1"`
	Command string `json:"command"`
	Output  string `json:"output"`
}
