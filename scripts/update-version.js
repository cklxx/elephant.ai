#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

// Colors for console output
const colors = {
    red: '\x1b[31m',
    green: '\x1b[32m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m',
    magenta: '\x1b[35m',
    cyan: '\x1b[36m',
    reset: '\x1b[0m'
};

function log(color, message) {
    console.log(`${colors[color]}${message}${colors.reset}`);
}

// Get new version from command line arguments
const newVersion = process.argv[2];

if (!newVersion) {
    log('red', '❌ Usage: node scripts/update-version.js <version>');
    log('yellow', '📋 Example: node scripts/update-version.js 0.0.3');
    process.exit(1);
}

// Validate version format (basic semver check)
if (!/^\d+\.\d+\.\d+(-.*)?$/.test(newVersion)) {
    log('red', `❌ Invalid version format: ${newVersion}`);
    log('yellow', '📋 Use semantic versioning format: X.Y.Z or X.Y.Z-suffix');
    process.exit(1);
}

const rootDir = path.join(__dirname, '..');

// Files to update with their update functions
const filesToUpdate = [
    // Main alex-code package
    {
        path: 'npm/alex-code/package.json',
        name: 'Main Package',
        update: (content) => {
            const pkg = JSON.parse(content);
            const oldVersion = pkg.version;
            pkg.version = newVersion;
            
            // Update optionalDependencies versions
            if (pkg.optionalDependencies) {
                Object.keys(pkg.optionalDependencies).forEach(dep => {
                    pkg.optionalDependencies[dep] = newVersion;
                });
            }
            
            log('blue', `  ${oldVersion} → ${newVersion}`);
            return JSON.stringify(pkg, null, 2) + '\n';
        }
    },
    
    // Platform-specific packages
    {
        path: 'npm/alex-linux-amd64/package.json',
        name: 'Linux AMD64 Package', 
        update: (content) => updatePlatformPackage(content, newVersion)
    },
    {
        path: 'npm/alex-linux-arm64/package.json',
        name: 'Linux ARM64 Package',
        update: (content) => updatePlatformPackage(content, newVersion)
    },
    {
        path: 'npm/alex-darwin-amd64/package.json', 
        name: 'Darwin AMD64 Package',
        update: (content) => updatePlatformPackage(content, newVersion)
    },
    {
        path: 'npm/alex-darwin-arm64/package.json',
        name: 'Darwin ARM64 Package', 
        update: (content) => updatePlatformPackage(content, newVersion)
    },
    {
        path: 'npm/alex-windows-amd64/package.json',
        name: 'Windows AMD64 Package',
        update: (content) => updatePlatformPackage(content, newVersion)
    },
    
    // Install script
    {
        path: 'npm/alex-code/install.js',
        name: 'Install Script',
        update: (content) => {
            const versionRegex = /const VERSION = '[^']+';/;
            const oldMatch = content.match(versionRegex);
            const oldVersion = oldMatch ? oldMatch[0].match(/'([^']+)'/)[1] : 'unknown';
            const updated = content.replace(versionRegex, `const VERSION = '${newVersion}';`);
            log('blue', `  ${oldVersion} → ${newVersion}`);
            return updated;
        }
    }
];

function updatePlatformPackage(content, version) {
    const pkg = JSON.parse(content);
    const oldVersion = pkg.version;
    pkg.version = version;
    log('blue', `  ${oldVersion} → ${version}`);
    return JSON.stringify(pkg, null, 2) + '\n';
}

// Main update process
log('cyan', '🚀 Starting version update process...');
log('magenta', `📦 Target Version: ${newVersion}`);
console.log('');

let updateCount = 0;
let errorCount = 0;

filesToUpdate.forEach(file => {
    const fullPath = path.join(rootDir, file.path);
    
    try {
        if (!fs.existsSync(fullPath)) {
            log('yellow', `⚠️  ${file.name}: File not found - ${file.path}`);
            return;
        }
        
        const content = fs.readFileSync(fullPath, 'utf8');
        const updated = file.update(content);
        
        fs.writeFileSync(fullPath, updated);
        log('green', `✅ ${file.name}: Updated`);
        updateCount++;
        
    } catch (error) {
        log('red', `❌ ${file.name}: ${error.message}`);
        errorCount++;
    }
});

console.log('');
log('cyan', '📊 Update Summary:');
log('green', `✅ Successfully updated: ${updateCount} files`);
if (errorCount > 0) {
    log('red', `❌ Failed updates: ${errorCount} files`);
}

if (errorCount === 0) {
    log('green', '🎉 All version updates completed successfully!');
    console.log('');
    log('yellow', '📝 Next steps:');
    log('blue', '1. Review changes: git diff');
    log('blue', '2. Test build: make build-all');
    log('blue', '3. Copy binaries: make copy-npm-binaries');
    log('blue', '4. Publish packages: make publish-npm');
} else {
    log('red', '⚠️  Some updates failed. Please check the errors above.');
    process.exit(1);
}