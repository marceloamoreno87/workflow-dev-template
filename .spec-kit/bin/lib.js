'use strict';
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const YAML = require('yaml');
const { execFileSync } = require('node:child_process');
const root = path.resolve(__dirname, '../..');
const wikiTask = 'Atualizar a LLM Wiki em .knowledge/ com as mudanças realizadas neste ciclo';
function git(...args) {
  return execFileSync('git', args, { cwd: root, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024,
    env: { ...process.env, OKF_AUTO_SYNC: '1' }, stdio: ['ignore', 'pipe', 'pipe'] }).trimEnd();
}
function safe(file) {
  if (typeof file !== 'string' || !file || path.isAbsolute(file) || file.includes('\\') || file.split('/').some(p => p === '..' || p === '.')) throw Error(`Caminho inválido: ${file}`);
  let current = root;
  for (const part of file.split('/')) {
    current = path.join(current, part);
    try { if (fs.lstatSync(current).isSymbolicLink()) throw Error(`Symlink não permitido: ${file}`); }
    catch (e) { if (e.code !== 'ENOENT') throw e; }
  }
  return path.join(root, file);
}
const read = file => fs.readFileSync(safe(file), 'utf8');
const write = (file, text) => { fs.mkdirSync(path.dirname(safe(file)), { recursive: true }); fs.writeFileSync(safe(file), text); };
const hash = text => crypto.createHash('sha256').update(text).digest('hex');
function document(text) {
  const doc = YAML.parseDocument(text, { uniqueKeys: true });
  if (doc.errors.length) throw Error(doc.errors.map(e => e.message).join('; '));
  return doc;
}
function metadata(file, kind = 'metadata') {
  const match = read(file).match(new RegExp(`<!-- spec-kit:${kind} -->\\s*\x60\x60\x60json\\s*([\\s\\S]*?)\x60\x60\x60`));
  if (!match) throw Error(`Bloco ${kind} ausente em ${file}`);
  return JSON.parse(match[1]);
}
function setMetadata(file, meta) {
  write(file, read(file).replace(/(<!-- spec-kit:metadata -->\s*```json\s*)[\s\S]*?(```)/, (_, a, b) => a + JSON.stringify(meta, null, 2) + '\n' + b));
}
function feature(explicit) {
  const file = explicit || JSON.parse(read('.specify/feature.json')).feature_directory;
  if (!/^specs\/(?:backlog\/|active\/|archive\/)?[a-zA-Z0-9-]+$/.test(file) || ['specs/backlog', 'specs/active', 'specs/archive'].includes(file)) throw Error('Feature inválida. Use uma pasta dentro de specs/.');
  safe(file); return file;
}
function head() { try { return git('rev-parse', '--verify', 'HEAD'); } catch { return null; } }
function tasks(file, complete = false) {
  const entries = [...read(`${file}/tasks.md`).matchAll(/^\s*- \[([ xX])\] (.+)$/gm)];
  const isWiki = t => t[2].includes(wikiTask);
  if (!entries.length || entries.filter(isWiki).length !== 1) throw Error('tasks.md precisa de uma tarefa da Wiki, exatamente uma vez.');
  if (complete && entries.some(t => t[1] === ' ' && !isWiki(t))) throw Error('Há tarefas de implementação pendentes.');
  return entries;
}
function validateQuality(meta, manifest = validateBundle()) {
  if (!meta || meta.status !== 'completed' || meta.tests_passed !== true) throw Error('Spec ainda não registra implementação e testes concluídos.');
  if (!/^human:[^\s]+$/.test(meta.approved_by || '')) throw Error('approved_by deve identificar a aprovação real como human:<id>.');
  const quality = meta.quality;
  if (!quality || !quality.tests || !quality.lint || !quality.types) throw Error('Evidências estruturadas de testes, lint e tipos estão ausentes.');
  for (const name of ['tests', 'lint', 'types']) {
    if (quality[name].passed !== true || typeof quality[name].command !== 'string' || !quality[name].command.trim()) throw Error(`Evidência inválida para ${name}.`);
  }
  const minimum = manifest.quality?.tests?.minimum_coverage_percent;
  if (!quality.coverage || quality.coverage.passed !== true || typeof quality.coverage.command !== 'string' || !quality.coverage.command.trim() ||
      typeof quality.coverage.percent !== 'number' || typeof minimum !== 'number' || quality.coverage.percent < minimum) throw Error(`Cobertura deve comprovar no mínimo ${minimum}%.`);
  return quality;
}
function validateBundle(overrides = new Map()) {
  const get = file => overrides.has(file) ? overrides.get(file) : read(file);
  const manifest = document(get('.knowledge/manifest.yaml')).toJS();
  if (!manifest || manifest.okf_version !== '0.2' || !Array.isArray(manifest.domains)) throw Error('Manifesto OKF inválido.');
  if (!manifest.system || typeof manifest.system.name !== 'string' || !manifest.system.name.trim() ||
      typeof manifest.system.version !== 'string' || !/^\d+\.\d+\.\d+$/.test(manifest.system.version) ||
      !manifest.system.stack || typeof manifest.system.stack !== 'object' || Array.isArray(manifest.system.stack)) throw Error('Seção system inválida no manifesto.');
  if (!/^\d+\.\d+\.\d+$/.test(manifest.knowledge_version || '') ||
      !(manifest.last_synced_commit === null || /^[0-9a-f]{40,64}$/.test(manifest.last_synced_commit || ''))) throw Error('Versão/commit de conhecimento inválido no manifesto.');
  if (!manifest.quality || typeof manifest.quality.tests?.minimum_coverage_percent !== 'number' ||
      manifest.quality.tests.minimum_coverage_percent < 0 || manifest.quality.tests.minimum_coverage_percent > 100 ||
      manifest.quality.lint?.required !== true || manifest.quality.types?.strict !== true) throw Error('Diretrizes de qualidade inválidas no manifesto.');
  const ids = new Set(); const paths = new Set();
  for (const d of manifest.domains) {
    if (!d || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(d.id || '') || ids.has(d.id) || paths.has(d.path)) throw Error('ID/caminho de domínio vazio ou duplicado.');
    safe(`.knowledge/${d.path}`);
    if (!d.path.startsWith('domains/') || !d.path.endsWith('.md')) throw Error(`Caminho de domínio inválido: ${d.path}`);
    if (!Array.isArray(d.depends_on) || new Set(d.depends_on).size !== d.depends_on.length || d.depends_on.includes(d.id)) throw Error(`depends_on inválido: ${d.id}`);
    if (!Array.isArray(d.source_paths) || !d.source_paths.length || d.source_paths.some(p => typeof p !== 'string' || !p.startsWith('src/') || /[*?]/.test(p))) throw Error(`source_paths inválido: ${d.id}`);
    for (const p of d.source_paths) safe(p);
    ids.add(d.id); paths.add(d.path);
  }
  for (const d of manifest.domains) {
    for (const dep of d.depends_on) if (!ids.has(dep)) throw Error(`Dependência desconhecida: ${dep}`);
    get(`.knowledge/${d.path}`);
  }
  const files = new Set(overrides.keys());
  function walk(dir) {
    for (const entry of fs.readdirSync(safe(dir), { withFileTypes: true })) {
      const file = `${dir}/${entry.name}`; safe(file);
      if (entry.isDirectory()) walk(file); else files.add(file);
    }
  }
  walk('.knowledge');
  for (const file of files) {
    if (!file.startsWith('.knowledge/') || !file.endsWith('.md')) continue;
    const text = get(file); const fm = text.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
    const name = path.basename(file);
    if (name === 'index.md' || name === 'log.md') {
      if (fm && (file !== '.knowledge/index.md' || Object.keys(document(fm[1]).toJS()).some(k => k !== 'okf_version'))) throw Error(`Frontmatter reservado inválido: ${file}`);
      if (file === '.knowledge/index.md' && (!fm || document(fm[1]).toJS().okf_version !== '0.2')) throw Error('index.md raiz deve declarar OKF 0.2.');
      if (name === 'log.md' && [...text.matchAll(/^##\s+(.+)$/gm)].some(m => !/^\d{4}-\d{2}-\d{2}$/.test(m[1]))) throw Error(`Cabeçalho de data inválido: ${file}`);
      continue;
    }
    if (!fm) throw Error(`Frontmatter ausente: ${file}`);
    const data = document(fm[1]).toJS();
    if (!data || typeof data.type !== 'string' || !data.type.trim()) throw Error(`type vazio/inválido: ${file}`);
    if (data.status && !['draft', 'stable', 'deprecated'].includes(data.status)) throw Error(`status inválido: ${file}`);
    if (data.sources && (!Array.isArray(data.sources) || data.sources.some(s => !s || typeof s.resource !== 'string' || !s.resource.trim()))) throw Error(`sources inválido: ${file}`);
    const actor = value => typeof value === 'string' && /^(?:human:|process:|[^\s/]+\/)[^\s]+$/.test(value);
    if (data.generated && (!actor(data.generated.by) || !data.generated.at || !Number.isFinite(Date.parse(data.generated.at)))) throw Error(`generated inválido: ${file}`);
    const verified = data.verified ? (Array.isArray(data.verified) ? data.verified : [data.verified]) : [];
    if (verified.some(v => !v || !actor(v.by) || !v.at || !Number.isFinite(Date.parse(v.at)))) throw Error(`verified inválido: ${file}`);
    if (data.stale_after && !Number.isFinite(Date.parse(data.stale_after))) throw Error(`stale_after inválido: ${file}`);
  }
  return manifest;
}
module.exports = { fs, path, root, YAML, git, safe, read, write, hash, document, metadata, setMetadata, feature, head, tasks, wikiTask, validateBundle, validateQuality };
