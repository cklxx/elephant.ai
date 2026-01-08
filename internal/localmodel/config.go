package localmodel

const (
	Provider              = "local"
	ModelID               = "functiongemma-270m-it"
	GGUFFileName          = "functiongemma-270m-it-BF16.gguf"
	RelativeModelPath     = "models/functiongemma/" + GGUFFileName
	RelativeTemplatePath  = "models/functiongemma/chat_template.jinja"
	BaseURL               = "http://127.0.0.1:11437/v1"
	DownloadURL           = "https://huggingface.co/unsloth/functiongemma-270m-it-GGUF/resolve/main/" + GGUFFileName
	DefaultContextSize    = 131072
	MinModelSizeBytes     = 50 * 1024 * 1024
	DefaultLlamaRelease   = "b7658"
	DefaultServerHost     = "127.0.0.1"
	DefaultServerPort     = "11437"
	LlamaServerBinaryName = "llama-server"
)
