import { readdir, readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const srcRoot = path.resolve(here, '..', 'src');
const featuresRoot = path.join(srcRoot, 'features');
const errors = [];
const sourceFiles = [];

async function walk(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) await walk(fullPath);
    else if (/\.(?:ts|tsx|js|jsx|mjs|css)$/.test(entry.name)) sourceFiles.push(fullPath);
  }
}

await walk(srcRoot);
const featureEntries = (await readdir(featuresRoot, { withFileTypes: true })).filter((entry) => entry.isDirectory());
for (const feature of featureEntries) {
  const indexPath = path.join(featuresRoot, feature.name, 'index.ts');
  try { await stat(indexPath); } catch { errors.push(`features/${feature.name}: missing public index.ts entry point`); }
}

for (const filename of sourceFiles) {
  const source = await readFile(filename, 'utf8');
  const relative = path.relative(srcRoot, filename).split(path.sep).join('/');
  if (/\bfrom\s*['"]@\/pages\//.test(source) || /\bfrom\s*['"]\.\/pages\//.test(source)) {
    errors.push(`${relative}: imports from the retired shared pages directory`);
  }
  if (/\bfrom\s*['"]@\/components\/(?:commands|graph|issues|plans|shelf|layout)\//.test(source)) {
    errors.push(`${relative}: imports an implementation file from the shared components directory; use the owning feature's public API`);
  }

  const ownerMatch = relative.match(/^features\/([^/]+)\//);
  const ownerFeature = ownerMatch?.[1];
  const importPattern = /(?:from\s*|import\s*\()\s*['"]@\/features\/([^/'"]+)(?:\/([^'"]+))?['"]/g;
  for (const match of source.matchAll(importPattern)) {
    const [, targetFeature, privatePath] = match;
    if (ownerFeature && ownerFeature !== targetFeature && privatePath) {
      errors.push(`${relative}: crosses feature boundary into features/${targetFeature}/${privatePath}; import from @/features/${targetFeature} instead`);
    }
    if (!ownerFeature && privatePath && relative !== 'features/README.md') {
      errors.push(`${relative}: app/shared code imports a private feature path; use @/features/${targetFeature}`);
    }
  }
}

const indexCSS = await readFile(path.join(srcRoot, 'index.css'), 'utf8');
if (indexCSS.length > 1800) errors.push(`index.css should remain an import manifest (currently ${indexCSS.length} characters)`);
if (!indexCSS.includes("./styles/tokens.css") || !indexCSS.includes("./styles/base.css")) errors.push('index.css must explicitly load design tokens and base styles');

if (errors.length) {
  for (const error of errors) console.error(`FAIL ${error}`);
  console.error(`\nFeature boundary check failed: ${errors.length} issue(s).`);
  process.exit(1);
}
console.log(`OK feature boundaries: ${featureEntries.length} feature modules have public entry points; ${sourceFiles.length} source/style files checked; global CSS entry stays modular.`);
