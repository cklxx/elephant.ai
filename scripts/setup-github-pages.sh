#!/bin/bash

# GitHub Pages Setup Script for Alex Project
# This script helps you set up automated deployment to GitHub Pages

set -e

echo "🚀 Alex Project - GitHub Pages Setup"
echo "===================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# Check if we're in a git repository
if [ ! -d ".git" ]; then
    error "This doesn't appear to be a Git repository."
    echo "Please run this script from the root of your Alex project."
    exit 1
fi

# Check if required files exist
echo "🔍 Checking required files..."

required_files=(
    ".github/workflows/deploy-pages.yml"
    "docs/index.html"
    "docs/manifest.json"
    "docs/sitemap.xml"
    "docs/robots.txt"
)

missing_files=()

for file in "${required_files[@]}"; do
    if [ -f "$file" ]; then
        success "Found $file"
    else
        error "Missing $file"
        missing_files+=("$file")
    fi
done

if [ ${#missing_files[@]} -gt 0 ]; then
    echo ""
    error "Missing required files. Please ensure you have:"
    for file in "${missing_files[@]}"; do
        echo "  - $file"
    done
    echo ""
    echo "Run the following to create the website:"
    echo "  make build"
    echo "  cd docs && ./deploy.sh"
    exit 1
fi

echo ""
success "All required files are present!"

# Check git remote
echo ""
echo "🔗 Checking Git configuration..."

if ! git remote -v | grep -q "github.com"; then
    warning "No GitHub remote found."
    echo ""
    echo "Please add your GitHub repository as origin:"
    echo "  git remote add origin https://github.com/yourusername/Alex-Code.git"
    echo ""
    read -p "Enter your GitHub repository URL (https://github.com/username/repo.git): " repo_url
    
    if [ -n "$repo_url" ]; then
        git remote add origin "$repo_url" 2>/dev/null || git remote set-url origin "$repo_url"
        success "Added GitHub remote: $repo_url"
    else
        error "Repository URL is required"
        exit 1
    fi
else
    repo_url=$(git remote get-url origin)
    success "GitHub remote found: $repo_url"
fi

# Extract username and repo name from URL
if [[ $repo_url =~ github\.com[:/]([^/]+)/([^/.]+) ]]; then
    username="${BASH_REMATCH[1]}"
    repo_name="${BASH_REMATCH[2]}"
    
    info "GitHub Username: $username"
    info "Repository Name: $repo_name"
    
    # Update sitemap and manifest with correct URLs
    echo ""
    echo "🔧 Updating configuration files..."
    
    # Update sitemap.xml
    if [ -f "docs/sitemap.xml" ]; then
        sed -i.backup "s|https://.*github.io/[^/]*/|https://${username}.github.io/${repo_name}/|g" docs/sitemap.xml
        rm -f docs/sitemap.xml.backup
        success "Updated sitemap.xml with correct URL"
    fi
    
    # Update HTML meta tags
    if [ -f "docs/index.html" ]; then
        sed -i.backup "s|https://.*github.io/[^/]*/|https://${username}.github.io/${repo_name}/|g" docs/index.html
        rm -f docs/index.html.backup
        success "Updated index.html with correct URLs"
    fi
else
    warning "Could not parse GitHub repository URL"
fi

# Check if main branch exists
echo ""
echo "🌿 Checking Git branches..."

current_branch=$(git branch --show-current)
info "Current branch: $current_branch"

if [ "$current_branch" != "main" ]; then
    warning "You're not on the 'main' branch."
    echo ""
    echo "GitHub Pages deployment is configured to trigger on pushes to 'main'."
    echo "Options:"
    echo "  1. Switch to main branch: git checkout main"
    echo "  2. Rename current branch: git branch -m main"
    echo "  3. Continue with current branch (you'll need to update the workflow)"
    echo ""
    read -p "Do you want to rename the current branch to 'main'? (y/N): " rename_branch
    
    if [[ $rename_branch =~ ^[Yy]$ ]]; then
        git branch -m main
        success "Renamed branch to 'main'"
        current_branch="main"
    fi
fi

# Commit and push changes
echo ""
echo "📤 Preparing to deploy..."

# Check if there are uncommitted changes
if ! git diff --quiet || ! git diff --cached --quiet; then
    warning "You have uncommitted changes."
    echo ""
    git status --short
    echo ""
    read -p "Do you want to commit these changes? (y/N): " commit_changes
    
    if [[ $commit_changes =~ ^[Yy]$ ]]; then
        git add .
        git commit -m "🌐 Setup GitHub Pages deployment

- Configure automated deployment workflow
- Update website URLs and configuration
- Ready for GitHub Pages deployment"
        success "Committed changes"
    else
        warning "Skipping commit. You may need to commit changes manually."
    fi
fi

# Push to GitHub
echo ""
read -p "Do you want to push to GitHub now? (y/N): " push_now

if [[ $push_now =~ ^[Yy]$ ]]; then
    echo "📤 Pushing to GitHub..."
    
    if git push -u origin "$current_branch"; then
        success "Successfully pushed to GitHub!"
    else
        error "Failed to push to GitHub"
        echo "You may need to authenticate or check your permissions."
        exit 1
    fi
else
    info "Skipping push. Remember to push when ready:"
    echo "  git push -u origin $current_branch"
fi

# Final instructions
echo ""
echo "🎉 Setup Complete!"
echo "=================="
echo ""
success "GitHub Pages deployment is configured!"
echo ""
echo "📋 Next Steps:"
echo ""
echo "1. 🔧 Enable GitHub Pages in your repository:"
echo "   • Go to https://github.com/$username/$repo_name/settings/pages"
echo "   • Under 'Source', select 'GitHub Actions'"
echo "   • Click 'Save'"
echo ""
echo "2. 🔑 Configure repository permissions:"
echo "   • Go to https://github.com/$username/$repo_name/settings/actions"
echo "   • Under 'Workflow permissions', select 'Read and write permissions'"
echo "   • Check 'Allow GitHub Actions to create and approve pull requests'"
echo "   • Click 'Save'"
echo ""
echo "3. 🚀 Trigger deployment:"
echo "   • Push any changes to the 'main' branch, or"
echo "   • Go to https://github.com/$username/$repo_name/actions"
echo "   • Run 'Deploy to GitHub Pages' workflow manually"
echo ""
echo "🌐 Your website will be available at:"
echo "   https://$username.github.io/$repo_name/"
echo ""
echo "📊 Monitor deployment:"
echo "   https://github.com/$username/$repo_name/actions"
echo ""
info "Deployment typically takes 2-5 minutes after pushing changes."
echo ""
success "Happy coding! 🚀"