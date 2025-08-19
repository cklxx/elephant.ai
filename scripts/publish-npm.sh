#!/bin/bash
set -e

echo "🚀 Starting smart npm publish process (with existing version detection)..."

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
NPM_DIR="$ROOT_DIR/npm"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to safely check if package exists on npm
check_package_exists_safe() {
    local package_name=$1
    local version=$2
    
    echo -e "${BLUE}🔍 Checking $package_name@$version...${NC}"
    
    # Method 1: Try npm view with specific version (most reliable)
    if npm view "$package_name@$version" version >/dev/null 2>&1; then
        echo -e "${YELLOW}✓ Found: $package_name@$version (npm view)${NC}"
        return 0
    fi
    
    # Method 2: Check all versions of the package
    local versions_output
    if versions_output=$(npm view "$package_name" versions --json 2>/dev/null); then
        if echo "$versions_output" | grep -q "\"$version\""; then
            echo -e "${YELLOW}✓ Found: $package_name@$version (in versions list)${NC}"
            return 0
        fi
    fi
    
    # Method 3: Try npm info (alternative to view)
    if npm info "$package_name@$version" version >/dev/null 2>&1; then
        echo -e "${YELLOW}✓ Found: $package_name@$version (npm info)${NC}"
        return 0
    fi
    
    echo -e "${GREEN}✗ Not found: $package_name@$version (ready to publish)${NC}"
    return 1
}

# Function to publish a package smartly
publish_package_smart() {
    local pkg_dir=$1
    local pkg_name=$2
    
    cd "$pkg_dir"
    local version=$(node -p "require('./package.json').version")
    
    echo -e "${BLUE}📦 Processing $pkg_name@$version${NC}"
    
    # Check if already published
    if check_package_exists_safe "$pkg_name" "$version"; then
        echo -e "${YELLOW}⏭️  Skipping $pkg_name@$version (already published)${NC}"
        return 0
    fi
    
    echo -e "${GREEN}🚀 Publishing $pkg_name@$version...${NC}"
    
    # Try to publish
    if npm publish --access public; then
        echo -e "${GREEN}✅ Successfully published $pkg_name@$version${NC}"
        return 0
    else
        # Capture error and check if it's about existing version
        local error_output
        error_output=$(npm publish --access public 2>&1 || true)
        
        if echo "$error_output" | grep -i -E "(already exists|cannot publish over|previously published|403 forbidden)"; then
            echo -e "${YELLOW}⚠️  Package $pkg_name@$version already exists (detected from error)${NC}"
            echo -e "${YELLOW}    Error details: ${error_output}${NC}"
            return 0  # Treat as success
        else
            echo -e "${RED}❌ Failed to publish $pkg_name@$version${NC}"
            echo -e "${RED}    Error: $error_output${NC}"
            return 1
        fi
    fi
}

# First, run the copy script to ensure binaries are up-to-date
echo -e "${BLUE}🔄 Ensuring binaries are copied to npm packages...${NC}"
"$ROOT_DIR/scripts/copy-npm-binaries.sh"

# Define all packages to publish
PLATFORM_DIRS=(
  "alex-linux-amd64"
  "alex-linux-arm64" 
  "alex-darwin-amd64"
  "alex-darwin-arm64"
  "alex-windows-amd64"
)

# Function to get package name from directory name
get_package_name() {
    local dir_name=$1
    echo "alex-code-${dir_name#alex-}"
}

# Step 1: Publish platform-specific packages
echo -e "${GREEN}📦 Step 1: Publishing platform-specific packages...${NC}"
published_count=0
failed_packages=()

for pkg_dir in "${PLATFORM_DIRS[@]}"; do
    pkg_name=$(get_package_name "$pkg_dir")
    echo ""
    if publish_package_smart "$NPM_DIR/$pkg_dir" "$pkg_name"; then
        ((published_count++))
    else
        failed_packages+=("$pkg_name")
    fi
done

echo ""
echo -e "${BLUE}📊 Platform packages summary:${NC}"
echo -e "${GREEN}  ✅ Successfully processed: $published_count/${#PLATFORM_DIRS[@]}${NC}"

if [ ${#failed_packages[@]} -gt 0 ]; then
    echo -e "${RED}  ❌ Failed packages: ${failed_packages[*]}${NC}"
    echo -e "${YELLOW}  ⚠️  Will attempt to continue if failures are due to existing packages${NC}"
    
    # Final verification of "failed" packages
    echo -e "${BLUE}🔄 Final verification of failed packages...${NC}"
    real_failures=()
    for pkg_name in "${failed_packages[@]}"; do
        # Convert package name back to directory name
        pkg_dir="${pkg_name/alex-code-/alex-}"
        
        if [[ -d "$NPM_DIR/$pkg_dir" ]]; then
            cd "$NPM_DIR/$pkg_dir"
            version=$(node -p "require('./package.json').version")
            if ! check_package_exists_safe "$pkg_name" "$version"; then
                real_failures+=("$pkg_name")
            fi
        fi
    done
    
    if [ ${#real_failures[@]} -gt 0 ]; then
        echo -e "${RED}❌ Cannot continue - real failures detected: ${real_failures[*]}${NC}"
        exit 1
    else
        echo -e "${GREEN}✅ All packages are actually available - continuing${NC}"
    fi
else
    echo -e "${GREEN}  🎉 All platform packages processed successfully!${NC}"
fi

# Step 2: Wait for propagation
echo ""
echo -e "${YELLOW}⏳ Waiting 30 seconds for NPM propagation...${NC}"
sleep 30

# Step 3: Publish main package
echo -e "${GREEN}📦 Step 2: Publishing main alex-code package...${NC}"
if ! publish_package_smart "$NPM_DIR/alex-code" "alex-code"; then
    echo -e "${RED}💥 Failed to publish main alex-code package${NC}"
    exit 1
fi

# Step 4: Final summary
echo ""
echo -e "${GREEN}🎉 NPM publication completed!${NC}"

cd "$NPM_DIR/alex-code"
main_version=$(node -p "require('./package.json').version")
echo -e "${GREEN}📦 Main package: alex-code@$main_version${NC}"
echo -e "${GREEN}🌐 Install with: npm install -g alex-code@$main_version${NC}"

# Optional: Quick verification
echo ""
echo -e "${BLUE}🔍 Quick verification...${NC}"
if check_package_exists_safe "alex-code" "$main_version"; then
    echo -e "${GREEN}✅ alex-code@$main_version is available on npm${NC}"
else
    echo -e "${YELLOW}⚠️  alex-code@$main_version may still be propagating${NC}"
fi

echo -e "${GREEN}🎊 Smart NPM publish process completed successfully!${NC}"