#!/bin/bash
set -e

echo "Starting npm publish process..."

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
NPM_DIR="$ROOT_DIR/npm"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check if package exists on npm
check_package_exists() {
    local package_name=$1
    local version=$2
    
    if npm view "$package_name@$version" version >/dev/null 2>&1; then
        return 0  # Package exists
    else
        return 1  # Package doesn't exist
    fi
}

# Function to publish a package with retry logic
publish_package() {
    local pkg_dir=$1
    local pkg_name=$2
    local max_retries=3
    local retry_delay=10
    
    cd "$pkg_dir"
    version=$(node -p "require('./package.json').version")
    
    echo -e "${YELLOW}--- Publishing $pkg_name@$version ---${NC}"
    
    # Check if already published
    if check_package_exists "$pkg_name" "$version"; then
        echo -e "${YELLOW}⚠️  Package $pkg_name@$version already exists, skipping${NC}"
        return 0
    fi
    
    # Attempt to publish with retries
    for ((i=1; i<=max_retries; i++)); do
        if npm publish --access public; then
            echo -e "${GREEN}✅ Successfully published $pkg_name@$version${NC}"
            return 0
        else
            echo -e "${RED}❌ Failed to publish $pkg_name@$version (attempt $i/$max_retries)${NC}"
            if [ $i -lt $max_retries ]; then
                echo -e "${YELLOW}⏳ Waiting ${retry_delay}s before retry...${NC}"
                sleep $retry_delay
            fi
        fi
    done
    
    echo -e "${RED}💥 Failed to publish $pkg_name after $max_retries attempts${NC}"
    return 1
}

# First, run the copy script to ensure binaries are up-to-date
echo "🔄 Copying binaries to npm packages..."
"$ROOT_DIR/scripts/copy-npm-binaries.sh"

PLATFORM_PACKAGES=(
  "alex-linux-amd64"
  "alex-linux-arm64"
  "alex-darwin-amd64"
  "alex-darwin-arm64"
  "alex-windows-amd64"
)

# Step 1: Publish all platform-specific packages
echo -e "${GREEN}📦 Step 1: Publishing platform-specific packages...${NC}"
platform_failures=()

for pkg in "${PLATFORM_PACKAGES[@]}"; do
    if ! publish_package "$NPM_DIR/$pkg" "$pkg"; then
        platform_failures+=("$pkg")
    fi
done

# Check if any platform packages failed
if [ ${#platform_failures[@]} -gt 0 ]; then
    echo -e "${RED}❌ Some platform packages failed to publish: ${platform_failures[*]}${NC}"
    echo -e "${YELLOW}⚠️  Cannot publish main package due to platform package failures${NC}"
    exit 1
fi

# Step 2: Wait for NPM propagation
echo -e "${YELLOW}⏳ Waiting 30 seconds for NPM propagation...${NC}"
sleep 30

# Step 3: Verify all platform packages are available
echo -e "${GREEN}🔍 Step 3: Verifying platform packages are available...${NC}"
verification_failures=()

for pkg in "${PLATFORM_PACKAGES[@]}"; do
    cd "$NPM_DIR/$pkg"
    version=$(node -p "require('./package.json').version")
    
    echo "Checking $pkg@$version..."
    if ! check_package_exists "$pkg" "$version"; then
        echo -e "${RED}❌ Package $pkg@$version not found on npm${NC}"
        verification_failures+=("$pkg@$version")
    else
        echo -e "${GREEN}✅ Package $pkg@$version is available${NC}"
    fi
done

if [ ${#verification_failures[@]} -gt 0 ]; then
    echo -e "${RED}❌ Some platform packages are not available: ${verification_failures[*]}${NC}"
    echo -e "${YELLOW}⚠️  Waiting additional 60 seconds for propagation...${NC}"
    sleep 60
fi

# Step 4: Publish the main package
echo -e "${GREEN}📦 Step 4: Publishing main alex-code package...${NC}"

if ! publish_package "$NPM_DIR/alex-code" "alex-code"; then
    echo -e "${RED}💥 Failed to publish main alex-code package${NC}"
    exit 1
fi

# Step 5: Final verification
echo -e "${GREEN}🎉 Step 5: Final verification...${NC}"
cd "$NPM_DIR/alex-code"
main_version=$(node -p "require('./package.json').version")

if check_package_exists "alex-code" "$main_version"; then
    echo -e "${GREEN}✅ Successfully verified alex-code@$main_version is available${NC}"
else
    echo -e "${YELLOW}⚠️  Main package may still be propagating...${NC}"
fi

echo -e "${GREEN}🎉 All npm packages have been published successfully!${NC}"
echo -e "${GREEN}📦 Main package: alex-code@$main_version${NC}"
echo -e "${GREEN}🌐 Install with: npm install -g alex-code@$main_version${NC}"
