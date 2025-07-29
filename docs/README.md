# Alex Documentation

Welcome to the Alex documentation. This directory contains essential documentation for the high-performance AI software engineering assistant.

## 📋 Available Documentation

### 🚀 Getting Started
- **[Quick Start Guide](guides/quickstart.md)** - Get up and running with Alex
- **[Tool Development Guide](guides/tool-development.md)** - Learn to develop custom tools

### 📊 Research & Planning
- **[Ultra Think Report](Ultra-Think-Report-Container-Mobile-Agent.md)** - 完整的云端沙箱与移动端开发调研报告
- **[Implementation Plan](implementation-plan.md)** - 实际可行的项目实施方案
- **[Context Engineering Report](context_engineering_july_2025_deep_report.md)** - 深度上下文工程研究报告

## 🔧 Configuration

Alex uses `alex-config.json` for configuration. The configuration file should be placed in your home directory (`~/.alex-config.json`) or in the current working directory.

### Basic Configuration Example
```json
{
  "baseURL": "https://api.openai.com/v1",
  "apiKey": "your-api-key-here",
  "model": "gpt-4",
  "maxTokens": 4000,
  "temperature": 0.7
}
```

## 🛠️ Development

For development information, refer to:
- **Main Project Documentation**: See `CLAUDE.md` in the project root
- **API Reference**: Available in the code documentation
- **Examples**: Check the `examples/` directory in the project root

## 🌐 GitHub Pages

This documentation is automatically deployed to GitHub Pages. The site structure:

- **Documentation**: Markdown files are automatically converted to web pages
- **Assets**: Static files like images and icons are served from the `assets/` directory
- **Web Resources**: Additional web resources are stored in the `web/` directory

## 📁 Directory Structure

```
docs/
├── index.html                                        # Main landing page
├── README.md                                         # This file
├── _config.yml                                       # Jekyll configuration
├── Ultra-Think-Report-Container-Mobile-Agent.md     # 云端沙箱调研报告
├── implementation-plan.md                            # 实际实施方案
├── context_engineering_july_2025_deep_report.md     # 上下文工程报告
├── assets/                                           # Static assets
│   └── favicon.svg
├── guides/                                           # Documentation guides
│   ├── quickstart.md
│   └── tool-development.md
└── web/                                              # Additional web resources
    ├── index.html      # Alternative landing page
    ├── manifest.json   # Web app manifest
    ├── robots.txt      # Search engine instructions
    └── sitemap.xml     # Site map
```

## 🚀 Local Development

To run the documentation site locally:

1. Install Jekyll and dependencies:
   ```bash
   cd docs
   bundle install
   ```

2. Serve the site locally:
   ```bash
   bundle exec jekyll serve
   ```

3. Open http://localhost:4000 in your browser

## 📖 Contributing

When contributing to documentation:

1. Keep it concise and practical
2. Include code examples where helpful
3. Test any commands or configurations
4. Follow the existing structure and style

For major changes, discuss first by opening an issue.