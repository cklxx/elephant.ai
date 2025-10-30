# Sandbox Environment

Pre-initialized development environment in the ALEX sandbox container.

## Node.js Environment

**Version:** 22.20.0
**Package Manager:** npm 10.9.3

### Globally Installed Packages

| Package | Description |
|---------|-------------|
| `typescript` | TypeScript compiler and language support |
| `ts-node` | Execute TypeScript files directly |
| `@types/node` | Node.js type definitions |
| `prettier` | Code formatter |
| `eslint` | JavaScript linter |
| `nodemon` | Auto-restart on file changes |
| `pm2` | Production process manager |
| `pnpm` | Fast, disk space efficient package manager |
| `yarn` | Alternative package manager |

### Usage Examples

```bash
# TypeScript compilation
tsc --init
tsc index.ts

# Run TypeScript directly
ts-node script.ts

# Format code
prettier --write "**/*.{js,ts,json}"

# Lint code
eslint src/

# Watch mode
nodemon app.js
```

## Python Environment

**Version:** 3.10.12
**Package Manager:** pip 25.2

### Installed Packages

#### Web & API Development
- `requests` - HTTP library
- `httpx` - Async HTTP client
- `aiohttp` - Async HTTP server/client
- `fastapi` - Modern web framework
- `uvicorn` - ASGI server
- `flask` - Micro web framework

#### Data Science
- `numpy` - Numerical computing
- `pandas` - Data analysis and manipulation
- `matplotlib` - Data visualization
- `scipy` - Scientific computing
- `scikit-learn` - Machine learning

#### Development Tools
- `pytest` - Testing framework
- `pytest-asyncio` - Async test support
- `black` - Code formatter
- `flake8` - Linter
- `mypy` - Static type checker
- `ipython` - Enhanced Python shell
- `jupyter` - Interactive notebooks

#### Utilities
- `python-dotenv` - .env file support
- `pyyaml` - YAML parser
- `click` - CLI framework
- `rich` - Rich text formatting
- `tqdm` - Progress bars

### Usage Examples

```bash
# Run tests
pytest tests/

# Format code
black .

# Type checking
mypy src/

# Start FastAPI server
uvicorn main:app --reload

# Interactive Python
ipython

# Jupyter notebook
jupyter notebook
```

## Environment Variables

- `PYTHONUNBUFFERED=1` - Real-time logging output
- `PYTHONDONTWRITEBYTECODE=1` - Prevent .pyc files
- `NODE_ENV=development` - Node.js development mode

## Rebuilding the Sandbox

To rebuild the sandbox with updated packages:

```bash
# Stop current sandbox
./deploy.sh down

# Rebuild sandbox image
docker-compose -f docker-compose.yml build sandbox

# Start services
./deploy.sh start
```

## Adding More Packages

### Node.js Packages

Edit `Dockerfile.sandbox` and add to the `npm install -g` command:

```dockerfile
RUN npm install -g --quiet \
    your-package@latest \
    && npm cache clean --force
```

### Python Packages

Edit `Dockerfile.sandbox` and add to the `pip3 install` command:

```dockerfile
RUN pip3 install --no-cache-dir \
    your-package
```

## Notes

- All packages are installed in the container, not on the host
- Changes require rebuilding the Docker image
- Base image: `ghcr.io/agent-infra/sandbox:latest`
- Workspace directory: `/workspace`
