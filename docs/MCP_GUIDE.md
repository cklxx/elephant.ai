# MCP (Model Context Protocol) Support in ALEX

## Overview

ALEX now supports the Model Context Protocol (MCP), an open standard for connecting AI assistants to external tools and data sources. MCP enables ALEX to extend its capabilities by integrating with external servers that provide additional tools.

## What is MCP?

The Model Context Protocol is a standardized way for AI applications to:
- **Discover** available tools from external servers
- **Execute** tool calls via a JSON-RPC 2.0 interface
- **Access** external data sources (files, databases, APIs)
- **Extend** functionality without modifying core code

**Specification**: https://modelcontextprotocol.io/specification

## Architecture

ALEX's MCP implementation follows the hexagonal architecture pattern:

```
┌─────────────────────────────────────────────────────────────┐
│                        ALEX Core                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Tool Registry (Ports)                    │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐     │   │
│  │  │   Builtin  │  │  Dynamic   │  │    MCP     │     │   │
│  │  │   Tools    │  │   Tools    │  │   Tools    │     │   │
│  │  └────────────┘  └────────────┘  └────────────┘     │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     MCP Registry                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  MCP Server  │  │  MCP Server  │  │  MCP Server  │      │
│  │  Instance 1  │  │  Instance 2  │  │  Instance 3  │      │
│  │              │  │              │  │              │      │
│  │  [Client]    │  │  [Client]    │  │  [Client]    │      │
│  │  [Process]   │  │  [Process]   │  │  [Process]   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
               ┌──────────────────────────────┐
               │   External MCP Servers       │
               │  (filesystem, github, etc.)  │
               └──────────────────────────────┘
```

### Components

1. **JSON-RPC Client** (`internal/mcp/jsonrpc.go`)
   - Implements JSON-RPC 2.0 protocol
   - Request/response marshaling
   - Error handling

2. **Process Manager** (`internal/mcp/process.go`)
   - Spawns and manages MCP server processes
   - Stdio communication
   - Automatic restart with exponential backoff
   - Health monitoring

3. **MCP Client** (`internal/mcp/client.go`)
   - Communicates with MCP servers over stdio
   - Initialize handshake
   - List and call tools
   - Protocol version: `2024-11-05`

4. **Tool Adapter** (`internal/mcp/tool_adapter.go`)
   - Adapts MCP tools to ALEX's `ToolExecutor` interface
   - Schema conversion (MCP → ALEX)
   - Result formatting

5. **MCP Registry** (`internal/mcp/registry.go`)
   - Discovers and loads MCP servers
   - Manages server lifecycle
   - Registers tools with ALEX
   - Health checking

6. **Configuration** (`internal/mcp/config.go`)
   - Parses `.mcp.json` files
   - Three-tier configuration (local > project > user)
   - Environment variable expansion

## Configuration

### Configuration Scopes

MCP servers are configured in `.mcp.json` files with three levels of priority:

1. **Local** (highest priority): `./.mcp.json` (current directory)
2. **Project**: `<git-root>/.mcp.json`
3. **User** (lowest priority): `~/.alex/.mcp.json`

Configuration is merged with local overriding project, and project overriding user.

### Configuration Format

```json
{
  "mcpServers": {
    "server-name": {
      "command": "executable-command",
      "args": ["arg1", "arg2"],
      "env": {
        "ENV_VAR": "${ENV_VAR}"
      },
      "disabled": false
    }
  }
}
```

**Fields:**
- `command` (required): Executable command to start the MCP server
- `args` (optional): Command-line arguments
- `env` (optional): Environment variables (supports `${VAR}` expansion)
- `disabled` (optional): Set to `true` to disable the server

### Example Configuration

See `.mcp.json.example` in the project root for a complete example.

## Usage

### CLI Commands

#### List MCP Servers

```bash
alex mcp list
```

Shows all configured MCP servers, their status, uptime, and capabilities.

#### Add MCP Server

```bash
alex mcp add <name> <command> [args...]
```

Example:
```bash
alex mcp add filesystem npx -y @modelcontextprotocol/server-filesystem /workspace
alex mcp add github npx -y @modelcontextprotocol/server-github
```

This adds the server to `.mcp.json` in the current directory.

#### Remove MCP Server

```bash
alex mcp remove <name>
```

Example:
```bash
alex mcp remove filesystem
```

#### List Tools

List all MCP tools:
```bash
alex mcp tools
```

List tools from a specific server:
```bash
alex mcp tools filesystem
```

#### Restart Server

```bash
alex mcp restart <name>
```

Example:
```bash
alex mcp restart filesystem
```

### Using MCP Tools

Once MCP servers are configured and running, their tools are automatically available to ALEX. The LLM can discover and use them during task execution.

MCP tool names are prefixed with `mcp__<server>__<tool>`, for example:
- `mcp__filesystem__read_file`
- `mcp__filesystem__write_file`
- `mcp__github__create_issue`

You can use them like any other tool:

```bash
alex "Read the file at /workspace/README.md using the MCP filesystem server"
```

## Available MCP Servers

### Official Servers

1. **@modelcontextprotocol/server-filesystem**
   - Read/write/search files
   - Installation: `npx -y @modelcontextprotocol/server-filesystem <directory>`

2. **@modelcontextprotocol/server-github**
   - Create issues, PRs, manage repositories
   - Requires: `GITHUB_TOKEN` environment variable
   - Installation: `npx -y @modelcontextprotocol/server-github`

3. **@modelcontextprotocol/server-postgres**
   - Query PostgreSQL databases
   - Installation: `npx -y @modelcontextprotocol/server-postgres <connection-string>`

4. **@modelcontextprotocol/server-puppeteer**
   - Browser automation
   - Installation: `npx -y @modelcontextprotocol/server-puppeteer`

### Custom Servers

You can create your own MCP servers following the specification at https://modelcontextprotocol.io/specification

## Troubleshooting

### Server Won't Start

1. Check if the command is in PATH:
   ```bash
   which npx
   ```

2. Verify the server works standalone:
   ```bash
   npx -y @modelcontextprotocol/server-filesystem /workspace
   ```

3. Check ALEX logs for errors:
   ```bash
   alex mcp list  # Shows status and last error
   ```

### Tools Not Available

1. Verify server is running:
   ```bash
   alex mcp list
   ```

2. Check server capabilities:
   ```bash
   alex mcp tools <server-name>
   ```

3. Restart the server:
   ```bash
   alex mcp restart <server-name>
   ```

### Environment Variables Not Expanding

Ensure environment variables are set before starting ALEX:

```bash
export GITHUB_TOKEN=your_token_here
alex interactive
```

In `.mcp.json`, use `${VAR}` syntax:
```json
{
  "env": {
    "GITHUB_TOKEN": "${GITHUB_TOKEN}"
  }
}
```

### Protocol Version Mismatch

ALEX uses MCP protocol version `2024-11-05`. If a server uses a different version, you'll see a warning in the logs. Most servers are backward compatible, but some features may not work.

## Development

### Running Tests

```bash
# Run all MCP tests
go test ./internal/mcp/... -v

# Run specific test
go test ./internal/mcp/ -run TestClient -v

# Run with coverage
go test ./internal/mcp/... -cover
```

### Creating Custom MCP Tools

1. Implement the MCP specification
2. Accept stdio communication
3. Support initialize handshake
4. Implement `tools/list` and `tools/call` methods

Example minimal server (pseudocode):
```javascript
const server = new MCPServer({
  name: "my-server",
  version: "1.0.0"
});

server.registerTool({
  name: "my_tool",
  description: "Does something useful",
  inputSchema: {
    type: "object",
    properties: {
      param1: { type: "string" }
    }
  },
  handler: async (params) => {
    return {
      content: [{ type: "text", text: "Result here" }]
    };
  }
});

server.listen();
```

## Performance Considerations

- **Startup Time**: MCP servers start in parallel during ALEX initialization
- **Tool Calls**: Add ~50-200ms latency due to IPC overhead
- **Memory**: Each MCP server runs as a separate process
- **Restart Policy**: Servers auto-restart on crash with exponential backoff (max 3 attempts)

## Security

- **Process Isolation**: Each MCP server runs in its own process
- **No Network Access**: Communication is strictly via stdio
- **Environment Variables**: Never log sensitive environment variables
- **Command Validation**: Commands are validated for invalid characters

## Limitations

- **Stdio Only**: Currently only stdio transport is supported (no HTTP/WebSocket)
- **No Batch Requests**: Single request/response only
- **Resources/Prompts**: Phase 1 focuses on tools; resources and prompts support coming later
- **No Streaming**: Tool results are buffered (max 1MB)

## Future Enhancements

- [ ] HTTP/WebSocket transport support
- [ ] MCP resources integration
- [ ] MCP prompts integration
- [ ] Batch request support
- [ ] Streaming tool results
- [ ] Server discovery/marketplace
- [ ] Performance metrics and monitoring

## References

- [MCP Specification](https://modelcontextprotocol.io/specification)
- [MCP Server Repository](https://github.com/modelcontextprotocol/servers)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
