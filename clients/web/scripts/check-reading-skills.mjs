import { readdir, readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, '..');
const skillsRoot = path.join(webRoot, 'src', 'features', 'reading-skills', 'skills');
const registryPath = path.join(webRoot, 'src', 'features', 'reading-skills', 'model', 'readingSkills.ts');
const assistantPath = path.join(webRoot, 'src', 'features', 'shelf', 'components', 'ShelfStudyAssistant.tsx');
const readerPagePath = path.join(webRoot, 'src', 'features', 'shelf', 'ShelfPDFReaderPage.tsx');
const companionSkillPath = path.join(webRoot, 'src', 'features', 'shelf', 'model', 'reading-companion.skill.md');
const studyStoragePath = path.join(webRoot, 'src', 'features', 'shelf', 'model', 'study-storage.ts');
const registry = await readFile(registryPath, 'utf8');
const assistantSource = await readFile(assistantPath, 'utf8');
const readerPageSource = await readFile(readerPagePath, 'utf8');
const companionSkillSource = await readFile(companionSkillPath, 'utf8');
const studyStorageSource = await readFile(studyStoragePath, 'utf8');
const errors = [];
const warnings = [];
const seenNames = new Set();
const skillDirs = await readdir(skillsRoot, { withFileTypes: true });
const files = [];

for (const entry of skillDirs.filter((item) => item.isDirectory()).sort((a, b) => a.name.localeCompare(b.name))) {
  const skillPath = path.join(skillsRoot, entry.name, 'SKILL.md');
  try { await stat(skillPath); } catch { continue; }
  const source = await readFile(skillPath, 'utf8');
  files.push({ id: entry.name });
  const frontmatterMatch = source.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n/);
  if (!frontmatterMatch) {
    errors.push(`${entry.name}: missing valid YAML frontmatter at file start`);
    continue;
  }
  const frontmatter = frontmatterMatch[1];
  const nameMatch = frontmatter.match(/^name:\s*([a-z0-9-]+)\s*$/m);
  const descMatch = frontmatter.match(/^description:\s*(.+)\s*$/m);
  if (!nameMatch) errors.push(`${entry.name}: name is missing or not lowercase kebab-case`);
  const name = nameMatch?.[1];
  if (name && name !== entry.name) errors.push(`${entry.name}: frontmatter name '${name}' must match directory name`);
  if (name && name.length > 64) errors.push(`${entry.name}: name exceeds 64 characters`);
  if (name && seenNames.has(name)) errors.push(`${entry.name}: duplicate skill name '${name}'`);
  if (name) seenNames.add(name);
  if (!descMatch || !descMatch[1].trim()) errors.push(`${entry.name}: description is required`);
  else if (descMatch[1].trim().length > 1024) errors.push(`${entry.name}: description exceeds 1024 characters`);
  const body = source.slice(frontmatterMatch[0].length).trim();
  if (!body) errors.push(`${entry.name}: instruction body is empty`);
  if (source.split(/\r?\n/).length > 500) warnings.push(`${entry.name}: SKILL.md exceeds the recommended 500 lines`);
  const isTranslation = entry.name === 'contextual-translation';
  if (!body.includes('## Output')) errors.push(`${entry.name}: required section '## Output' is missing`);
  if (!isTranslation && !/^## .*?(?:verification gate|quality gate)/im.test(body)) errors.push(`${entry.name}: required verification/quality gate section is missing`);
  if (isTranslation && !body.includes('## Hard failures')) errors.push(`${entry.name}: translation hard-failure checks are missing`);
  if (entry.name !== 'core-reliability' && !/Never invent|Do not invent|Do not claim|Never claim|Do not silently|Never silently|fabricat|hallucinat/i.test(body)) {
    errors.push(`${entry.name}: needs at least one explicit anti-fabrication / honesty guardrail`);
  }
  const importRef = `../skills/${entry.name}/SKILL.md?raw`;
  if (!registry.includes(importRef)) errors.push(`${entry.name}: SKILL.md is not imported in readingSkills.ts`);
  if (!registry.includes(`id: '${entry.name}'`)) errors.push(`${entry.name}: skill is not registered in READING_SKILLS`);
}

const core = files.find((item) => item.id === 'core-reliability');
const translation = files.find((item) => item.id === 'contextual-translation');
if (!core) errors.push('core-reliability: mandatory core contract is missing');
if (!translation) errors.push('contextual-translation: mandatory translation skill is missing');
if (!/id: 'core-reliability'[^\n]*alwaysOn: true/.test(registry)) errors.push('registry: core-reliability must be always on');
if (!/id: 'contextual-translation'[^\n]*alwaysOn: true/.test(registry)) errors.push('registry: contextual-translation must be always on');
if (!registry.includes('skill.alwaysOn || skill.defaultEnabled')) errors.push('registry: default selection must include mandatory skills');
if (!assistantSource.includes('activeSkillInstructions')) errors.push('Tutor/Mimi: selected skill instructions are no longer injected');
if (!assistantSource.includes('coreReliabilitySkill.replace')) errors.push('Tutor/Mimi: isolated repair calls must receive the core reliability contract');
if (!readerPageSource.includes('CORE_RELIABILITY_CONTRACT')) errors.push('Translate: mandatory core reliability contract is not imported/injected');
for (const required of [
  'Page-level argument reading and negation fidelity',
  'Never silently drop or reverse negations',
  'treat earlier agent messages as fallible claims',
]) {
  if (!companionSkillSource.includes(required)) errors.push(`MIMI: shared reading companion contract is missing "${required}"`);
}
if (!assistantSource.includes('## Your role: MIMI — the primary Shelf reading companion')) errors.push('MIMI: chat mode must retain the primary MIMI identity');
if (!studyStorageSource.includes('idios.shelf.page-insight.v6.')) errors.push('MIMI: page insight cache must invalidate notes from the previous prompt contract');

const evalPath = path.join(webRoot, '..', '..', 'tests', 'fixtures', 'reading-skill-evaluations.json');
try {
  const evalCases = JSON.parse(await readFile(evalPath, 'utf8'));
  if (!Array.isArray(evalCases) || evalCases.length < 8) errors.push('evals: expected at least eight evaluation cases');
  else {
    const known = new Set(files.map((item) => item.id));
    const coveredSkills = new Set();
    for (const [index, item] of evalCases.entries()) {
      if (!item.id || !item.prompt || !item.context || !item.pass_criteria?.length || !item.critical_failures?.length) errors.push(`evals case ${index + 1}: id, prompt, context, pass_criteria, and critical_failures are required`);
      if (item.skill && !known.has(item.skill)) errors.push(`evals case ${index + 1}: unknown skill '${item.skill}'`);
      if (item.skill && known.has(item.skill)) coveredSkills.add(item.skill);
    }
    for (const id of known) if (!coveredSkills.has(id)) errors.push(`evals: no behavior-evaluation case covers skill '${id}'`);
  }
} catch (error) {
  errors.push(`evals: cannot load/parse READING_SKILL_EVALUATIONS.json (${error instanceof Error ? error.message : String(error)})`);
}

for (const warning of warnings) console.warn(`WARN ${warning}`);
for (const error of errors) console.error(`FAIL ${error}`);
if (errors.length) {
  console.error(`\nReading skills check failed: ${errors.length} error(s), ${files.length} skill(s) scanned.`);
  process.exit(1);
}
console.log(`OK reading skills: ${files.length} registered skills, required frontmatter/sections, anti-fabrication rules, mandatory skills, and evaluation fixtures validated.`);
