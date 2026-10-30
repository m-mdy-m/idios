import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// Resolve the repository root from this script, not process.cwd(). pnpm runs
// package scripts from clients/web, while the checks below inspect Go sources
// at the repository root. This also works when invoked from another directory.
const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../../..');
const failures = [];
const read = (path) => readFileSync(join(root, path), 'utf8');
const mustExist = (path) => { try { return read(path); } catch { failures.push(`missing ${path}`); return ''; } };
const app = mustExist('internal/application/service.go');
const ports = mustExist('internal/ports/store.go');
const requiredServices = [
  ['repositories.go', 'RepositoryService', 'RepositoryStore'],
  ['projects.go', 'ProjectService', 'ProjectStore'],
  ['issues.go', 'IssueService', 'issueServiceStore'],
  ['pull_requests.go', 'PullRequestService', 'pullRequestServiceStore'],
  ['concepts.go', 'ConceptService', 'ConceptStore'],
  ['planning_metadata.go', 'PlanningMetadataService', 'PlanningMetadataStore'],
  ['graph.go', 'GraphService', 'GraphStore'],
  ['activity.go', 'ActivityService', 'ActivityStore'],
  ['notes.go', 'NotesService', 'NotesStore'],
  ['quotes.go', 'QuotesService', 'QuotesStore'],
  ['profile.go', 'ProfileService', 'ProfileStore'],
  ['jobs.go', 'JobsService', 'JobsStore'],
  ['notifications.go', 'NotificationsService', 'NotificationStore'],
  ['settings.go', 'SettingsService', 'SettingsStore'],
  ['plans.go', 'PlanService', 'PlanStore'],
  ['shelf.go', 'ShelfService', 'shelfServiceStore'],
];
for (const [file, type, port] of requiredServices) {
  const path = `internal/application/${file}`;
  const source = mustExist(path);
  if (!source.includes(`type ${type} struct`)) failures.push(`${path} must declare ${type}`);
  if (!source.includes(`ports.${port}`) && !source.includes(`${port}`)) failures.push(`${path} must use its narrow ${port} contract`);
  if (/store\s+ports\.Store\b/.test(source)) failures.push(`${path} must not depend on aggregate ports.Store`);
}
if (!app.includes('func New(store ports.Store) *Service')) failures.push('application facade must keep the stable New(ports.Store) constructor');
for (const [, , port] of requiredServices) {
  if (/^[A-Z]/.test(port) && !ports.includes(`type ${port} interface`)) failures.push(`ports/store.go must declare ${port}`);
}
const servicePorts = mustExist('internal/application/service_ports.go');
for (const port of ['issueServiceStore', 'pullRequestServiceStore', 'shelfServiceStore']) {
  if (!servicePorts.includes(`type ${port} interface`)) failures.push(`application/service_ports.go must declare ${port}`);
}
const serverDir = join(root, 'clients/server');
const serverFiles = readdirSync(serverDir).filter((name) => /^handlers_[a-z_]+\.go$/.test(name));
if (serverFiles.length < 8) failures.push('HTTP handlers must stay split by domain instead of returning to one server.go blob');
const serverLines = read('clients/server/server.go').split(/\r?\n/).length;
if (serverLines > 900) failures.push(`clients/server/server.go is ${serverLines} lines; lifecycle/routing/middleware should remain focused`);
const allHandlerText = serverFiles.map((name) => read(`clients/server/${name}`)).join('\n');
if (!allHandlerText.includes('func (s *Server) getShelf(')) failures.push('Shelf HTTP adapter must be in a feature handler file');
if (!allHandlerText.includes('func (s *Server) getRepositories(')) failures.push('repository HTTP adapter must be in a feature handler file');
const featureRoot = join(root, 'clients/web/src/features');
let featureDirs = 0;
for (const entry of readdirSync(featureRoot)) {
  const full = join(featureRoot, entry);
  if (!statSync(full).isDirectory()) continue;
  if (existsSync(join(full, 'index.ts'))) featureDirs++;
}
console.log(`Service boundaries: ${requiredServices.length} domain service contracts, ${serverFiles.length} HTTP handler groups, ${serverLines}-line server lifecycle/routing file; ${featureDirs} feature folders detected.`);
if (failures.length) { console.error(failures.map((value) => `FAIL ${value}`).join('\n')); process.exit(1); }
console.log('OK domain service and HTTP adapter boundaries.');
