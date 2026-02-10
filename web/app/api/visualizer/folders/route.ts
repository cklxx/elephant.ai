import { NextRequest, NextResponse } from 'next/server';
import { readdir, stat } from 'fs/promises';
import { join } from 'path';

export const dynamic = 'force-dynamic';

interface FolderInfo {
  path: string;
  name: string;
  fileCount: number;
  totalLines: number;
  depth: number;
}

const EXCLUDED_DIRS = new Set([
  'node_modules',
  '.git',
  '.next',
  'dist',
  'build',
  'out',
  'coverage',
  '.turbo',
  '.cache',
  'logs',
]);

const CODE_EXTENSIONS = new Set([
  '.ts', '.tsx', '.js', '.jsx', '.go', '.py', '.rs',
  '.java', '.cpp', '.c', '.h', '.hpp', '.cs', '.rb',
  '.php', '.swift', '.kt', '.scala', '.sh', '.bash',
  '.sql', '.md', '.json', '.yaml', '.yml', '.toml',
]);

async function countLinesInFile(filePath: string): Promise<number> {
  try {
    const fs = await import('fs/promises');
    const content = await fs.readFile(filePath, 'utf-8');
    return content.split('\n').length;
  } catch {
    return 0;
  }
}

async function scanDirectory(
  dirPath: string,
  basePath: string,
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
      if (entry.isFile()) {
        const ext = entry.name.substring(entry.name.lastIndexOf('.'));
        if (CODE_EXTENSIONS.has(ext)) {
          fileCount++;
          const filePath = join(dirPath, entry.name);
          totalLines += await countLinesInFile(filePath);
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
      if (entry.isDirectory() && !EXCLUDED_DIRS.has(entry.name) && !entry.name.startsWith('.')) {
        const subPath = join(dirPath, entry.name);
        const subFolders = await scanDirectory(subPath, basePath, maxDepth, currentDepth + 1);
        folders.push(...subFolders);
      }
    }

    return folders;
  } catch (error) {
    console.error(`Error scanning ${dirPath}:`, error);
    return [];
  }
}

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const workspaceParam = searchParams.get('workspace');

    // Use provided workspace or try to detect from CWD
    const workspace = workspaceParam || process.cwd();
    const maxDepth = parseInt(searchParams.get('depth') || '3', 10);

    console.log(`[Visualizer] Scanning workspace: ${workspace} (max depth: ${maxDepth})`);

    const folders = await scanDirectory(workspace, workspace, maxDepth);

    // Sort by file count (descending) for better initial view
    folders.sort((a, b) => b.fileCount - a.fileCount);

    return NextResponse.json({
      workspace,
      folders,
      count: folders.length,
      scannedAt: new Date().toISOString(),
    });
  } catch (error) {
    console.error('[Visualizer] Error scanning folders:', error);
    return NextResponse.json(
      { error: 'Failed to scan workspace folders' },
      { status: 500 }
    );
  }
}
