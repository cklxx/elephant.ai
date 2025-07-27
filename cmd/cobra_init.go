package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// newInitCommand 创建初始化命令
func newInitCommand(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "🚀 生成项目文档 ALEX.md",
		Long: `自动分析项目并生成 ALEX.md 文档文件。

该命令等价于执行：
  alex "分析当前项目并生成完整的 ALEX.md 项目文档"

示例:
  alex init                          # 生成 ALEX.md 项目文档`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 初始化CLI
			if err := cli.initialize(cmd); err != nil {
				return fmt.Errorf("failed to initialize CLI: %w", err)
			}

			// 获取当前工作目录和项目名称
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			projectName := filepath.Base(workDir)

			// Build detailed ALEX.md generation prompt
			prompt := fmt.Sprintf(`You are a professional project analyst. Your task is to analyze the project "%s" and generate a comprehensive ALEX.md documentation file.

# CRITICAL INSTRUCTIONS:
1. **THIS IS NOT ABOUT CREATING CONVERSATION MEMORY** - You are creating project documentation
2. **OUTPUT MUST BE A MARKDOWN FILE** - Generate actual ALEX.md file content
3. **DO NOT CREATE SHORT-TERM MEMORY** - This is a documentation generation task

# Task Workflow:

## Step 1: Deep Project Analysis
Use the following tools to comprehensively analyze the project:
- file_list to explore project structure  
- file_read to examine key files (README, main.go, config files, core modules)
- grep to search for patterns, features, and technologies used
- Understand the project's purpose, architecture, and key features
- Identify build system, testing approach, and usage patterns
- Analyze the codebase to understand design principles and architecture

## Step 2: Generate ALEX.md Documentation
Create a comprehensive documentation file "ALEX.md" with complete sections:

### Required Sections:
- **Project Overview** - Description and purpose of %s
- **Essential Development Commands** - Actual build, test, and usage commands
- **Architecture Overview** - Core components and modules description
- **Built-in Tools and Features** - List of available tools/features
- **Security Features** - Security measures and protections
- **Performance Characteristics** - Performance metrics and features
- **Code Principles and Design Philosophy** - Core design principles
- **Naming Guidelines** - Code naming conventions
- **Architectural Principles** - Key architectural decisions
- **Current Status** - Current development status
- **Testing Instructions** - How to run tests

## Step 3: Write the ALEX.md File
Use file_update or file_write to create the file "ALEX.md" with:
- Complete markdown content
- Professional documentation quality
- Clear structure and formatting
- Practical usage examples
- Comprehensive project insights

# CRITICAL REQUIREMENTS:
1. **GENERATE ACTUAL FILE** - Must create "ALEX.md" file with documentation content
2. **NO CONVERSATION MEMORY** - This is pure documentation generation
3. **ANALYZE FIRST** - Thoroughly examine the codebase before writing
4. **PROFESSIONAL QUALITY** - Documentation should be comprehensive and useful

Start analysis and file generation immediately! Working directory: %s`, projectName, projectName, workDir)

			// 直接使用 single prompt 模式，复用整体流程
			return cli.runSinglePrompt(prompt)
		},
	}

	return cmd
}
