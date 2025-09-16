const https = require('https');
const fs = require('fs');
const path = require('path');
const { exec } = require('child_process');

const REPO = 'cklxx/Alex-Code';

// Dynamically read version from package.json to ensure consistency
function getVersion() {
    try {
        const packageJson = require('./package.json');
        return packageJson.version;
    } catch (e) {
        console.warn('Warning: Could not read package.json version, using fallback');
        return '0.0.2';
    }
}

const VERSION = getVersion();

const BIN_DIR = path.join(__dirname, 'bin');
const BIN_PATH = path.join(BIN_DIR, 'alex');

// Ensure we can write to the bin directory
function ensureBinDirectory() {
    if (!fs.existsSync(BIN_DIR)) {
        fs.mkdirSync(BIN_DIR, { recursive: true });
    }
}

function getPlatform() {
    const platform = process.platform;
    const arch = process.arch;

    if (platform === 'darwin' && arch === 'arm64') return 'darwin-arm64';
    if (platform === 'darwin' && arch === 'x64') return 'darwin-amd64';
    if (platform === 'linux' && arch === 'arm64') return 'linux-arm64';
    if (platform === 'linux' && arch === 'x64') return 'linux-amd64';
    if (platform === 'win32' && arch === 'x64') return 'windows-amd64';

    return null;
}

function getExePath(platform) {
    const exe = platform.startsWith('windows') ? 'alex.exe' : 'alex';
    return path.join(BIN_DIR, exe);
}

function download(url, dest) {
    return new Promise((resolve, reject) => {
        const request = https.get(url, (response) => {
            if (response.statusCode === 302 || response.statusCode === 301) {
                return download(response.headers.location, dest).then(resolve).catch(reject);
            }
            if (response.statusCode !== 200) {
                return reject(new Error(`Failed to download file: ${response.statusCode} from ${url}`));
            }
            const file = fs.createWriteStream(dest);
            response.pipe(file);
            file.on('finish', () => file.close(resolve));
        }).on('error', (err) => {
            fs.unlink(dest, () => reject(err));
        });
    });
}

function findBinaryInNodeModules() {
    const platform = getPlatform();
    if (!platform) return null;

    const packageName = `alex-code-${platform}`;
    console.log(`Looking for platform-specific package: ${packageName}`);
    
    try {
        // Resolve the package.json of the platform-specific package
        const packageJsonPath = require.resolve(`${packageName}/package.json`);
        const packageDir = path.dirname(packageJsonPath);
        const binPath = platform.startsWith('windows')
            ? path.join(packageDir, 'bin', 'alex.exe')
            : path.join(packageDir, 'bin', 'alex');

        console.log(`Checking binary path: ${binPath}`);
        if (fs.existsSync(binPath)) {
            console.log(`✓ Found binary in ${packageName}`);
            return binPath;
        } else {
            console.log(`✗ Binary not found at ${binPath}`);
        }
    } catch (e) {
        console.log(`✗ Package ${packageName} not found: ${e.message}`);
    }
    return null;
}


async function main() {
    const platform = getPlatform();
    if (!platform) {
        console.error('Unsupported platform.');
        process.exit(1);
    }

    ensureBinDirectory();

    const finalExePath = getExePath(platform);
    let binaryPath = findBinaryInNodeModules();

    if (binaryPath) {
        // Copy the binary from the optional dependency to the local bin directory
        fs.copyFileSync(binaryPath, finalExePath);
    } else {
        // Fallback to downloading from GitHub
        console.log('Optional dependency not found, falling back to GitHub download.');
        console.log('Note: This fallback requires a GitHub release with binary assets.');
        
        // Try different version formats for GitHub releases
        const possibleTags = [`v${VERSION}`, VERSION, 'latest'];
        const binaryName = `alex-${platform}${platform.startsWith('windows') ? '.exe' : ''}`;
        
        let downloadSuccess = false;
        for (const tag of possibleTags) {
            const url = `https://github.com/${REPO}/releases/download/${tag}/${binaryName}`;
            console.log(`Trying to download from ${url}`);
            
            try {
                await download(url, finalExePath);
                console.log('✓ Download complete from GitHub.');
                downloadSuccess = true;
                break;
            } catch (error) {
                console.log(`✗ Failed to download from ${url}: ${error.message}`);
            }
        }
        
        if (!downloadSuccess) {
            console.error('❌ All GitHub download attempts failed.');
            console.error('Please ensure:');
            console.error('1. A GitHub release exists with the correct version tag');
            console.error('2. Binary assets are uploaded to the release');
            console.error(`3. Binary naming follows: alex-${platform}${platform.startsWith('windows') ? '.exe' : ''}`);
            process.exit(1);
        }
    }

    if (process.platform !== 'win32') {
        console.log('Making binary executable...');
        fs.chmodSync(finalExePath, '755');
    }

    // Manually create symlink for global installation
    // Check if we're in a global installation context
    const isGlobalInstall = process.env.npm_config_global === 'true' ||
                           __dirname.includes('/lib/node_modules/');

    if (isGlobalInstall) {
        // Calculate global bin directory more accurately
        let globalBinDir;
        if (process.env.npm_config_prefix) {
            globalBinDir = path.join(process.env.npm_config_prefix, 'bin');
        } else {
            // Use npm's convention: if we're in /installation/lib/node_modules, bin is in /installation/bin
            const nodeModulesPath = __dirname;
            const installationPath = nodeModulesPath.replace(/\/lib\/node_modules\/.*$/, '');
            globalBinDir = path.join(installationPath, 'bin');
        }

        const symlinkPath = path.join(globalBinDir, 'alex');

        console.log('🔗 Attempting to create global symlink...');
        console.log('  From:', finalExePath);
        console.log('  To:', symlinkPath);

        try {
            // Remove existing symlink if it exists
            if (fs.existsSync(symlinkPath)) {
                fs.unlinkSync(symlinkPath);
                console.log('  ✓ Removed existing symlink');
            }

            // Create new symlink
            fs.symlinkSync(finalExePath, symlinkPath);
            console.log('✅ Global symlink created successfully!');
        } catch (e) {
            console.log('⚠️ Could not create global symlink:', e.message);
            console.log('💡 You can manually run:', finalExePath);
            console.log('💡 Or add this to your PATH:', path.dirname(finalExePath));
        }
    } else {
        console.log('ℹ️ Local installation detected - no global symlink needed');
    }

    console.log('Alex-Code installed successfully!');
    console.log('✓ Binary installed at:', finalExePath);
    console.log('✓ You can now use: alex');
}

main();
