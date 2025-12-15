#!/usr/bin/env node

const fs = require('node:fs/promises');
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

async function loadSkills(skillsDir) {
  const entries = await fs.readdir(skillsDir, { withFileTypes: true });
  const mdFiles = entries
    .filter((entry) => entry.isFile() && entry.name.toLowerCase().endsWith('.md'))
    .map((entry) => entry.name)
    .sort((a, b) => a.localeCompare(b));

  const skills = [];

  for (const fileName of mdFiles) {
    const fullPath = path.join(skillsDir, fileName);
    const raw = await fs.readFile(fullPath, 'utf8');
    const { frontmatter, body } = splitFrontmatter(raw);
    const meta = parseFrontmatter(frontmatter);
    const fallbackName = fileName.replace(/\.md$/i, '');
    const name = (meta.name || fallbackName).trim() || fallbackName;
    const title = extractTitle(body) || name;
    const description = (meta.description || '').trim();

    skills.push({
      name,
      title,
      description,
      markdown: body.trim(),
      sourcePath: `skills/${fileName}`,
    });
  }

  return skills.sort((a, b) => a.name.localeCompare(b.name));
}

async function main() {
  const repoRoot = path.resolve(__dirname, '..', '..');
  const skillsDir = process.env.ALEX_SKILLS_DIR
    ? path.resolve(process.env.ALEX_SKILLS_DIR)
    : path.join(repoRoot, 'skills');

  const outputDir = path.join(repoRoot, 'web', 'lib', 'generated');
  const outputFile = path.join(outputDir, 'skillsCatalog.json');

  const skills = await loadSkills(skillsDir);
  const payload = { skills };

  await fs.mkdir(outputDir, { recursive: true });
  await fs.writeFile(outputFile, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');

  console.log(`Generated ${path.relative(repoRoot, outputFile)} (${skills.length} skills)`);
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
