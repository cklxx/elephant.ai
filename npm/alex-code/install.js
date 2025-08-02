const https = require('https');
const fs = require('fs');
const path = require('path');
const { exec } = require('child_process');

const REPO = 'cklxx/Alex-Code';
const VERSION = '1.0.1'; // Note: This is the package version, not the git tag.

const BIN_DIR = path.join(__dirname, 'bin');
const BIN_PATH = path.join(BIN_DIR, 'alex');

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

    const packageName = `@alex-code/${platform}`;
    try {
        // Resolve the package.json of the platform-specific package
        const packageJsonPath = require.resolve(`${packageName}/package.json`);
        const packageDir = path.dirname(packageJsonPath);
        const binPath = platform.startsWith('windows')
            ? path.join(packageDir, 'bin', 'alex.exe')
            : path.join(packageDir, 'bin', 'alex');

        if (fs.existsSync(binPath)) {
            console.log(`Found binary in ${packageName}`);
            return binPath;
        }
    } catch (e) {
        // package not found
    }
    return null;
}


async function main() {
    const platform = getPlatform();
    if (!platform) {
        console.error('Unsupported platform.');
        process.exit(1);
    }

    if (!fs.existsSync(BIN_DIR)) {
        fs.mkdirSync(BIN_DIR);
    }

    const finalExePath = getExePath(platform);
    let binaryPath = findBinaryInNodeModules();

    if (binaryPath) {
        // Copy the binary from the optional dependency to the local bin directory
        fs.copyFileSync(binaryPath, finalExePath);
    } else {
        // Fallback to downloading from GitHub
        console.log('Optional dependency not found, falling back to GitHub download.');
        const gitHubTag = `v${VERSION}`;
        const binaryName = `alex-${platform}${platform.startsWith('windows') ? '.exe' : ''}`;
        const url = `https://github.com/${REPO}/releases/download/${gitHubTag}/${binaryName}`;

        console.log(`Downloading from ${url}`);
        try {
            await download(url, finalExePath);
            console.log('Download complete.');
        } catch (error) {
            console.error('Error downloading from GitHub:', error);
            console.error('Please check if the release and assets exist.');
            process.exit(1);
        }
    }

    if (process.platform !== 'win32') {
        console.log('Making binary executable...');
        fs.chmodSync(finalExePath, '755');
    }

    console.log('Alex-Code installed successfully!');
}

main();
