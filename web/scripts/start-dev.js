#!/usr/bin/env node
const { createServer } = require('net');
const { spawn, execSync } = require('child_process');

const DEFAULT_PORT = 3000;
const MAX_ATTEMPTS = 20;
const preferredPort = Number(process.env.PORT) || DEFAULT_PORT;
const PORT_RELEASE_ATTEMPTS = 20;
const PORT_RELEASE_INTERVAL_MS = 250;
const turboDisabled = process.env.DISABLE_TURBO === '1';

function isPortAvailable(port) {
  return new Promise((resolve, reject) => {
    const tester = createServer()
      .once('error', (err) => {
        if (err.code === 'EADDRINUSE') {
          resolve(false);
        } else {
          reject(err);
        }
      })
      .once('listening', () => {
        tester.close(() => resolve(true));
      });

    tester.listen(port, '0.0.0.0');
  });
}

async function findAvailablePort(startPort, attempts = MAX_ATTEMPTS) {
  for (let offset = 0; offset < attempts; offset += 1) {
    const port = startPort + offset;
    if (await isPortAvailable(port)) {
      return port;
    }
  }

  throw new Error(`Unable to find an open port after checking ${attempts} options starting from ${startPort}.`);
}

function parsePidsFromNetstat(output, port) {
  const lines = output.split(/\r?\n/).filter(Boolean);
  const pids = new Set();

  lines.forEach((line) => {
    const parts = line.trim().split(/\s+/);
    if (parts.length < 5) {
      return;
    }

    const localAddress = parts[1];
    const pid = parts[parts.length - 1];

    if (localAddress && localAddress.endsWith(`:${port}`)) {
      pids.add(Number(pid));
    }
  });

  return Array.from(pids).filter((pid) => Number.isFinite(pid));
}

function getProcessIdsForPort(port) {
  try {
    if (process.platform === 'win32') {
      const result = execSync(`netstat -ano | findstr :${port}`, {
        stdio: ['ignore', 'pipe', 'ignore'],
        encoding: 'utf8',
      });
      return parsePidsFromNetstat(result, port);
    }

    const result = execSync(`lsof -ti tcp:${port} -sTCP:LISTEN`, {
      stdio: ['ignore', 'pipe', 'ignore'],
      encoding: 'utf8',
    });

    return result
      .split(/\r?\n/)
      .map((value) => Number(value.trim()))
      .filter((pid) => Number.isFinite(pid));
  } catch (error) {
    return [];
  }
}

async function tryKillProcessOnPort(port) {
  const pids = getProcessIdsForPort(port);

  if (!pids.length) {
    return false;
  }

  console.log(`Attempting to stop processes ${pids.join(', ')} on port ${port}...`);

  pids.forEach((pid) => {
    try {
      process.kill(pid, 'SIGTERM');
    } catch (error) {
      if (error.code !== 'ESRCH') {
        console.warn(`Unable to terminate process ${pid}: ${error.message}`);
      }
    }
  });

  for (let attempt = 0; attempt < PORT_RELEASE_ATTEMPTS; attempt += 1) {
    await new Promise((resolve) => setTimeout(resolve, PORT_RELEASE_INTERVAL_MS));
    if (await isPortAvailable(port)) {
      console.log(`Successfully released port ${port}.`);
      return true;
    }
  }

  console.warn(`Processes on port ${port} did not exit in time.`);
  return false;
}

async function preparePort(preferred) {
  if (await isPortAvailable(preferred)) {
    return preferred;
  }

  console.log(`Port ${preferred} is currently in use.`);
  if (await tryKillProcessOnPort(preferred)) {
    return preferred;
  }

  const fallbackPort = await findAvailablePort(preferred + 1);
  console.log(`Falling back to available port ${fallbackPort}.`);
  return fallbackPort;
}

(async () => {
  try {
    const port = await preparePort(preferredPort);

    const nextCommand = process.platform === 'win32' ? 'next.cmd' : 'next';
    const devArgs = ['dev', '-p', String(port)];

    if (!turboDisabled) {
      devArgs.splice(1, 0, '--turbo');
    }

    console.log(
      turboDisabled
        ? 'Starting Next.js dev server (webpack fallback, turbo disabled via DISABLE_TURBO=1).'
        : 'Starting Next.js dev server with the Rust-based Turbopack compiler...'
    );

    const devProcess = spawn(nextCommand, devArgs, {
      stdio: 'inherit',
      env: { ...process.env, PORT: String(port) },
    });

    devProcess.on('exit', (code) => {
      process.exit(code ?? 0);
    });
  } catch (error) {
    console.error(error.message || error);
    process.exit(1);
  }
})();
