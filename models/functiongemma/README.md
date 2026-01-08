# FunctionGemma assets

This directory vendors the local FunctionGemma model used by the default
"local" LLM provider.

Contents:
- `functiongemma-270m-it-BF16.gguf`: GGUF weights (BF16, no quantization).
- `chat_template.jinja`: Chat template extracted from
  `unsloth/functiongemma-270m-it` `tokenizer_config.json`.

Source:
- Model: https://huggingface.co/unsloth/functiongemma-270m-it-GGUF
- Template: https://huggingface.co/unsloth/functiongemma-270m-it
- License: Gemma (see https://ai.google.dev/gemma/terms)

SHA256:
- `functiongemma-270m-it-BF16.gguf`: 2d9469b8ea30c0aaa73f4119ca52426acc2edbf34cda4847f11644f4d0cd0193

Notes:
- The GGUF file is tracked via Git LFS. If you clone without LFS, run
  `git lfs pull` or let the runtime auto-download the weights.
