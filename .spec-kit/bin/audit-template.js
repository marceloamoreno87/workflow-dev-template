#!/usr/bin/env node
'use strict';
const { fs, safe, read, validateBundle } = require('./lib');

// Exclusivo do repositório template; projetos consumidores usam audit-workflow.
const failures = [];
function check(condition, message) { if (!condition) failures.push(message); }
function walk(dir, allowed) {
  for (const entry of fs.readdirSync(safe(dir), { withFileTypes: true })) {
    const file = `${dir}/${entry.name}`;
    safe(file);
    if (entry.isDirectory()) walk(file, allowed);
    else check(allowed.has(file) || (entry.name === '.gitkeep' && read(file).trim() === ''), `Artefato não permitido: ${file}`);
  }
}
try {
  const manifest = validateBundle();
  walk('specs', new Set());
  walk('src', new Set());
  walk('.knowledge', new Set(['.knowledge/index.md', '.knowledge/manifest.yaml']));
  check(!fs.existsSync(safe('.specify/feature.json')), 'Contexto gerado: .specify/feature.json');
  check(read('.specify/memory/constitution.md') === read('.specify/templates/constitution-template.md'), 'Constituição preenchida: .specify/memory/constitution.md');
  check(manifest.system.name === '[PROJECT_NAME]' && manifest.system.version === '0.0.0', 'Identidade de projeto preenchida no manifesto');
  check(manifest.knowledge_version === '0.0.0', 'knowledge_version deve ser 0.0.0');
  check(manifest.last_synced_commit === null, 'last_synced_commit deve ser null');
  check(manifest.domains.length === 0, 'Manifesto possui domínios preenchidos');
  check(/<!-- auto-sync:domains:start -->\s*<!-- auto-sync:domains:end -->/.test(read('.knowledge/index.md')), 'Índice contém domínios gerados');
  for (const file of ['WORKFLOW-REVIEW.md', 'WORKFLOW-MANUAL-TEST.md']) check(!fs.existsSync(safe(file)), `Relatório de sessão: ${file}`);
} catch (error) { failures.push(error.message); }
if (failures.length) {
  for (const failure of failures) console.error(`FAIL  ${failure}`);
  process.exitCode = 1;
} else console.log('Template limpo: sem specs, conhecimento compilado ou contexto de execução.');
