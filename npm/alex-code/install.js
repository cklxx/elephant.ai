const https = require('https');
const fs = require('fs');
const path = require('path');
const { exec } = require('child_process');

const REPO = 'cklxx/Alex-Code';
// This version should be kept in sync with the releases
const VERSION = 'v1.0.1';

const BIN_DIR = path.join(__dirname, 'bin');

function getPlatform() {
    const platform = process.platform;
    const arch = process.arch;

    if (platform === 'darwin' && arch === 'arm64') return 'darwin-arm64';
    if (platform === 'darwin' && arch === 'x64') return 'darwin-amd64';
    if (platform === 'linux' && arch === 'arm64') return 'linux-arm64';
    if (platform === 'linux' && arch === 'x64') return 'linux-amd64';
    if (platform === 'win32' && arch === 'x64') return 'windows-amd64.exe';

    console.error(`Unsupported platform: ${platform} ${arch}`);
    process.exit(1);
}

function getBinaryPath() {
    const platform = process.platform;
    let exe = 'alex';
    if (platform === 'win32') {
        exe = 'alex.exe';
    }
    return path.join(BIN_DIR, exe);
}

function download(url, dest) {
    return new Promise((resolve, reject) => {
        const request = https.get(url, (response) => {
            if (response.statusCode === 302 || response.statusCode === 301) {
                // Handle redirect
                download(response.headers.location, dest).then(resolve).catch(reject);
                return;
            }

            if (response.statusCode !== 200) {
                reject(new Error(`Failed to download file: ${response.statusCode} from ${url}`));
                return;
            }

            const file = fs.createWriteStream(dest);
            response.pipe(file);
            file.on('finish', () => {
                file.close(resolve);
            });
        });

        request.on('error', (err) => {
            fs.unlink(dest, () => reject(err));
        });
    });
}

async function main() {
    const platform = getPlatform();
    const binaryName = `alex-${platform}`;
    const url = `https://github.com/${REPO}/releases/download/${VERSION}/${binaryName}`;

    if (!fs.existsSync(BIN_DIR)) {
        fs.mkdirSync(BIN_DIR);
    }

    const binPath = getBinaryPath();

    console.log(`Downloading ${binaryName} from ${url}`);

    try {
        await download(url, binPath);
        console.log('Download complete.');

        if (process.platform !== 'win32') {
            console.log(`Making binary executable at ${binPath}...`);
            fs.chmodSync(binPath, '755');
        }

        console.log('Alex-Code installed successfully!');
    } catch (error) {
        console.error('Error installing Alex-Code:', error);
        process.exit(1);
    }
}

main();
