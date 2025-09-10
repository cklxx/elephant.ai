# ALEX Architecture Documentation

## 📚 Documentation Navigation

### 🏗️ Core Architecture Analysis
- [ALEX Architecture Analysis Report](architecture/ALEX_ARCHITECTURE_ANALYSIS.md) - Core architecture report with system overview and component analysis
- [ALEX Detailed Architecture Map](architecture/ALEX_DETAILED_ARCHITECTURE.md) - Ultimate architecture blueprint with complete system landscape

### 📊 Architecture Diagrams
- [System Overview](diagrams/system_overview.md) - High-level system architecture overview
- [Data Flow](diagrams/data_flow.md) - Detailed data flow architecture  
- [ReAct Cycle](diagrams/react_cycle.md) - Core ReAct execution process
- [Tool Ecosystem](diagrams/tool_ecosystem.md) - Complete tool system architecture
- [Complete Architecture](diagrams/complete_architecture.md) - Ultimate complete architecture diagram
- [ReAct Mindmap](diagrams/react_mindmap.md) - ReAct cognitive architecture mindmap

## 🎯 Quick Start

### View Architecture Overview
```bash
# View core architecture analysis
cat docs/architecture/ALEX_ARCHITECTURE_ANALYSIS.md

# View complete architecture map
cat docs/architecture/ALEX_DETAILED_ARCHITECTURE.md
```

### Render Architecture Diagrams
All diagrams use Mermaid format and can be viewed through:

1. **GitHub Online Rendering** - View `.md` files directly on GitHub
2. **VS Code** - Install Mermaid Preview extension  
3. **Online Tools** - Copy to [mermaid.live](https://mermaid.live) for viewing
4. **CLI Tools** - Use `mmdc` command line tool to generate images

### Generate PNG Images
If mermaid-cli is installed:
```bash
# Install mermaid-cli
npm install -g @mermaid-js/mermaid-cli

# Generate system overview diagram
mmdc -i docs/diagrams/system_overview.md -o docs/images/system_overview.png

# Generate all diagrams
for file in docs/diagrams/*.md; do
    filename=$(basename "$file" .md)
    mmdc -i "$file" -o "docs/images/${filename}.png"
done
```

## 🏗️ ALEX Architecture Features

### Core Components
- **🤖 ReactAgent** - ReAct architecture core engine with Think-Act-Observe cycle
- **🧠 LLM Abstraction Layer** - Multi-model support with DeepSeek Chat + DeepSeek R1  
- **🔧 Tool Ecosystem** - 13+ built-in tools + MCP protocol external tools
- **💾 Session Management** - Persistent storage with recovery and context compression
- **📈 SWE-Bench Evaluation** - Standardized evaluation framework with batch processing

### Technology Stack
- **Language**: Go 1.24 - High-performance, concise, concurrency-friendly
- **CLI**: Cobra + Viper - Powerful command-line parsing and configuration management
- **UI**: Bubble Tea + Lipgloss - Elegant terminal user interface
- **Protocol**: JSON-RPC 2.0 (MCP) - Standardized tool communication protocol
- **Storage**: File system - Simple and reliable session persistence

### Design Philosophy
> **Keep it simple and clear, add nothing without necessity**

- Simple and clear architectural design, avoiding over-engineering
- Interface-driven design with loose coupling and high cohesion
- Production-grade reliability with complete error handling and recovery mechanisms
- Terminal-native experience with streaming responses and real-time feedback

## 🎨 Diagram Specifications

### Diagram Types
- **System Diagrams** - Show component relationships and hierarchical structures
- **Flow Charts** - Display data flows and execution processes  
- **Sequence Diagrams** - Show interaction sequences between components
- **Mind Maps** - Display conceptual hierarchies and classification relationships

### Diagram Standards
- **Color Coding** - Different colors distinguish different layers
- **Icon Semantics** - Intuitive emoji icons represent component types
- **Arrow Direction** - Indicates data flow and dependency relationships
- **Group Layout** - Related components grouped together with clear hierarchy

## 📋 Documentation Maintenance

### Update Process
1. Update relevant architecture documentation after source code changes
2. Use AST analysis tools to verify architectural changes
3. Update Mermaid diagrams to reflect new architecture
4. Regenerate image files (if needed)

### Contribution Guidelines
- Keep documentation synchronized with code
- Use consistent diagram styles and color standards
- Add clear explanations and comments
- Provide multiple viewing formats

---

**ALEX** - *Agile Light Easy Xpert Code Agent*  
🚀 Production-ready terminal-native AI programming assistant