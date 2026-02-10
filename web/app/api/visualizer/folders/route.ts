import { NextRequest, NextResponse } from 'next/server';
import { readdir, stat, readFile } from 'fs/promises';
import { join, resolve } from 'path';
import { existsSync } from 'fs';

export const dynamic = 'force-dynamic';

interface FolderInfo {
  path: string;
  name: string;
  fileCount: number;
  totalLines: number;
  depth: number;
}

// Base exclusions (always ignore)
const BASE_EXCLUDED_DIRS = new Set([
  'node_modules',
  '.git',
  '.next',
  'dist',
  'build',
  'out',
  'coverage',
  '.turbo',
  '.cache',
]);

const CODE_EXTENSIONS = new Set([
  '.ts', '.tsx', '.js', '.jsx', '.go', '.py', '.rs',
  '.java', '.cpp', '.c', '.h', '.hpp', '.cs', '.rb',
  '.php', '.swift', '.kt', '.scala', '.sh', '.bash',
  '.sql', '.md', '.json', '.yaml', '.yml', '.toml',
]);

// Simple gitignore pattern matching
function shouldIgnore(path: string, patterns: string[]): boolean {
  const normalizedPath = path.replace(/\\/g, '/');

  for (const pattern of patterns) {
    if (!pattern || pattern.startsWith('#')) continue;

    // Remove leading/trailing whitespace
    const cleanPattern = pattern.trim();
    if (!cleanPattern) continue;

    // Directory pattern (ends with /)
    if (cleanPattern.endsWith('/')) {
      const dirPattern = cleanPattern.slice(0, -1);
      if (normalizedPath.includes(`/${dirPattern}/`) || normalizedPath.endsWith(`/${dirPattern}`)) {
        return true;
      }
    }

    // Exact match or contains
    if (normalizedPath.includes(cleanPattern) || normalizedPath.endsWith(cleanPattern)) {
      return true;
    }

    // Wildcard patterns (basic support)
    if (cleanPattern.includes('*')) {
      const regexPattern = cleanPattern
        .replace(/\./g, '\\.')
        .replace(/\*/g, '.*');
      const regex = new RegExp(regexPattern);
      if (regex.test(normalizedPath)) {
        return true;
      }
    }
  }

  return false;
}

async function parseGitignore(basePath: string): Promise<string[]> {
  const gitignorePath = join(basePath, '.gitignore');

  if (!existsSync(gitignorePath)) {
    return [];
  }

  try {
    const content = await readFile(gitignorePath, 'utf-8');
    return content
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !line.startsWith('#'));
  } catch (error) {
    console.error('[Visualizer] Error reading .gitignore:', error);
    return [];
  }
}

async function countLinesInFile(filePath: string): Promise<number> {
  try {
    const content = await readFile(filePath, 'utf-8');
    return content.split('\n').length;
  } catch {
    return 0;
  }
}

async function scanDirectory(
  dirPath: string,
  basePath: string,
  gitignorePatterns: string[],
  maxDepth: number = 3,
  currentDepth: number = 0
): Promise<FolderInfo[]> {
  if (currentDepth >= maxDepth) return [];

  try {
    const entries = await readdir(dirPath, { withFileTypes: true });
    const folders: FolderInfo[] = [];

    let fileCount = 0;
    let totalLines = 0;

    // Process files in current directory
    for (const entry of entries) {
      const fullPath = join(dirPath, entry.name);
      const relativePath = fullPath.replace(basePath, '').replace(/^\//, '');

      // Skip if in gitignore
      if (shouldIgnore(relativePath, gitignorePatterns)) {
        continue;
      }

      if (entry.isFile()) {
        const ext = entry.name.substring(entry.name.lastIndexOf('.'));
        if (CODE_EXTENSIONS.has(ext)) {
          fileCount++;
          totalLines += await countLinesInFile(fullPath);
        }
      }
    }

    // Add current folder if it has files
    const relativePath = dirPath.replace(basePath, '') || '/';
    if (fileCount > 0) {
      folders.push({
        path: relativePath,
        name: relativePath === '/' ? 'root' : relativePath.split('/').pop() || relativePath,
        fileCount,
        totalLines,
        depth: currentDepth,
      });
    }

    // Recursively scan subdirectories
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;

      const fullPath = join(dirPath, entry.name);
      const relativePath = fullPath.replace(basePath, '').replace(/^\//, '');

      // Skip base exclusions
      if (BASE_EXCLUDED_DIRS.has(entry.name) || entry.name.startsWith('.')) {
        continue;
      }

      // Skip if in gitignore
      if (shouldIgnore(relativePath, gitignorePatterns)) {
        continue;
      }

      const subFolders = await scanDirectory(
        fullPath,
        basePath,
        gitignorePatterns,
        maxDepth,
        currentDepth + 1
      );
      folders.push(...subFolders);
    }

    return folders;
  } catch (error) {
    console.error(`[Visualizer] Error scanning ${dirPath}:`, error);
    return [];
  }
}

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);

    // Find project root directory
    let workspace = process.cwd();

    // If running from web/ subdirectory, go up one level
    if (workspace.endsWith('/web') || workspace.endsWith('\\web')) {
      workspace = resolve(workspace, '..');
    }

    // Try to find project root by looking for markers
    const possibleRoots = [
      workspace,
      resolve(workspace, '..'),
      resolve(workspace, '../..'),
    ];

    for (const root of possibleRoots) {
      // Look for go.mod first (elephant.ai is a Go project)
      if (existsSync(join(root, 'go.mod'))) {
        workspace = root;
        break;
      }
      // Fallback to .git
      if (existsSync(join(root, '.git'))) {
        workspace = root;
        break;
      }
    }

    // Allow override via query param
    if (searchParams.get('workspace')) {
      workspace = searchParams.get('workspace')!;
    }

    const maxDepth = parseInt(searchParams.get('depth') || '3', 10);

    console.log(`[Visualizer] Scanning workspace: ${workspace} (max depth: ${maxDepth})`);

    // Parse .gitignore
    const gitignorePatterns = await parseGitignore(workspace);
    console.log(`[Visualizer] Loaded ${gitignorePatterns.length} gitignore patterns`);

    // Scan directories
    const folders = await scanDirectory(workspace, workspace, gitignorePatterns, maxDepth);

    // Sort by size (file count + line count) for better initial view
    folders.sort((a, b) => {
      const sizeA = a.fileCount + a.totalLines / 100;
      const sizeB = b.fileCount + b.totalLines / 100;
      return sizeB - sizeA;
    });

    return NextResponse.json({
      workspace,
      folders,
      count: folders.length,
      scannedAt: new Date().toISOString(),
      gitignorePatterns: gitignorePatterns.length,
    });
  } catch (error) {
    console.error('[Visualizer] Error scanning folders:', error);
    return NextResponse.json(
      { error: 'Failed to scan workspace folders' },
      { status: 500 }
    );
  }
}
