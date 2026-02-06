#!/usr/bin/env node

const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');

function stripQuotes(value) {
  const trimmed = String(value ?? '').trim();
  if (!trimmed) return '';
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function splitFrontmatter(markdown) {
  const raw = typeof markdown === 'string' ? markdown : '';
  if (!raw.startsWith('---')) {
    return { frontmatter: '', body: raw };
  }

  const lines = raw.split(/\r?\n/);
  let endIndex = -1;
  for (let i = 1; i < lines.length; i += 1) {
    if (lines[i].trim() === '---') {
      endIndex = i;
      break;
    }
  }

  if (endIndex === -1) {
    return { frontmatter: '', body: raw };
  }

  const frontmatter = lines.slice(1, endIndex).join('\n');
  const body = lines.slice(endIndex + 1).join('\n');
  return { frontmatter, body };
}

function parseFrontmatter(frontmatter) {
  const meta = {};
  const raw = typeof frontmatter === 'string' ? frontmatter : '';
  raw.split(/\r?\n/).forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) return;
    const match = trimmed.match(/^([A-Za-z0-9_-]+)\s*:\s*(.*)$/);
    if (!match) return;
    const key = match[1];
    const value = stripQuotes(match[2]);
    if (!key) return;
    meta[key] = value;
  });
  return meta;
}

function extractTitle(markdownBody) {
  const raw = typeof markdownBody === 'string' ? markdownBody : '';
  const lines = raw.split(/\r?\n/);
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('# ')) {
      return trimmed.slice(2).trim();
    }
  }
  return '';
}

async function discoverSkillFiles(skillsDir) {
  const entries = await fs.readdir(skillsDir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    if (entry.isDirectory()) {
      for (const candidate of ['SKILL.md', 'SKILL.mdx']) {
        const candidatePath = path.join(skillsDir, entry.name, candidate);
        const stat = await fs.stat(candidatePath).catch(() => null);
        if (stat?.isFile()) {
          files.push(candidatePath);
          break;
        }
      }
      continue;
    }

  }

  return files.sort((a, b) => a.localeCompare(b));
}

async function statPath(targetPath) {
  return fs.stat(targetPath).catch(() => null);
}

async function hasSkillDefinition(skillDir) {
  for (const candidate of ['SKILL.md', 'SKILL.mdx']) {
    const candidatePath = path.join(skillDir, candidate);
    const stat = await statPath(candidatePath);
    if (stat?.isFile()) {
      return true;
    }
  }
  return false;
}

async function copyDirectory(sourceDir, targetDir) {
  await fs.mkdir(targetDir, { recursive: true });
  const entries = await fs.readdir(sourceDir, { withFileTypes: true });
  for (const entry of entries) {
    const sourcePath = path.join(sourceDir, entry.name);
    const targetPath = path.join(targetDir, entry.name);
    if (entry.isDirectory()) {
      await copyDirectory(sourcePath, targetPath);
      continue;
    }
    if (entry.isFile()) {
      await fs.copyFile(sourcePath, targetPath);
    }
  }
}

async function copyMissingSkills(repoSkillsDir, homeSkillsDir) {
  const entries = await fs.readdir(repoSkillsDir, { withFileTypes: true });
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    const sourceSkillDir = path.join(repoSkillsDir, entry.name);
    if (!(await hasSkillDefinition(sourceSkillDir))) continue;

    const targetSkillDir = path.join(homeSkillsDir, entry.name);
    const targetStat = await statPath(targetSkillDir);
    if (targetStat) continue;

    await copyDirectory(sourceSkillDir, targetSkillDir);
  }
}

async function ensureHomeSkills(repoRoot, homeSkillsDir) {
  await fs.mkdir(homeSkillsDir, { recursive: true });

  const repoSkillsDir = path.join(repoRoot, 'skills');
  const repoStat = await statPath(repoSkillsDir);
  if (!repoStat?.isDirectory()) {
    return;
  }
  await copyMissingSkills(repoSkillsDir, homeSkillsDir);
}

async function resolveSkillsDir(repoRoot) {
  const envSkillsDir = stripQuotes(process.env.ALEX_SKILLS_DIR || '');
  if (envSkillsDir) {
    return path.resolve(envSkillsDir);
  }

  const homeDir = os.homedir();
  if (!homeDir) {
    return path.join(repoRoot, 'skills');
  }

  const homeSkillsDir = path.join(homeDir, '.alex', 'skills');
  await ensureHomeSkills(repoRoot, homeSkillsDir);
  return homeSkillsDir;
}

async function loadSkills(skillsDir, repoRoot) {
  const skillFiles = await discoverSkillFiles(skillsDir);
  const skills = [];

  for (const filePath of skillFiles) {
    const raw = await fs.readFile(filePath, 'utf8');
    const { frontmatter, body } = splitFrontmatter(raw);
    const meta = parseFrontmatter(frontmatter);
    const fileName = path.basename(filePath);
    const fallbackName = fileName.replace(/\.mdx?$/i, '');
    const name = (meta.name || fallbackName).trim() || fallbackName;
    const title = extractTitle(body) || name;
    const description = (meta.description || '').trim();
    const sourcePath = path
      .relative(repoRoot, filePath)
      .split(path.sep)
      .join('/');

    skills.push({
      name,
      title,
      description,
      markdown: body.trim(),
      sourcePath,
    });
  }

  return skills.sort((a, b) => a.name.localeCompare(b.name));
}

async function main() {
  const repoRoot = path.resolve(__dirname, '..', '..');
  const skillsDir = await resolveSkillsDir(repoRoot);

  const outputDir = path.join(repoRoot, 'web', 'lib', 'generated');
  const outputFile = path.join(outputDir, 'skillsCatalog.json');

  const skills = await loadSkills(skillsDir, repoRoot);
  const payload = { skills };

  await fs.mkdir(outputDir, { recursive: true });
  await fs.writeFile(outputFile, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');

  console.log(`Generated ${path.relative(repoRoot, outputFile)} (${skills.length} skills)`);
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
