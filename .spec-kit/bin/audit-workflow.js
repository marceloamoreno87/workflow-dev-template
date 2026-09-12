#!/usr/bin/env node
'use strict';
const { fs, path, root, read, safe, document, validateBundle } = require('./lib');

let failures = 0;
const pass = message => console.log(`PASS  ${message}`);
const fail = message => { failures++; console.error(`FAIL  ${message}`); };
const warn = message => console.warn(`WARN  ${message}`);
function check(condition, message) { condition ? pass(message) : fail(message); }
function files(dir) {
  const result = [];
  function walk(current) {
    for (const entry of fs.readdirSync(safe(current), { withFileTypes: true })) {
      const file = `${current}/${entry.name}`;
      if (entry.isDirectory()) walk(file); else result.push(file.slice(dir.length + 1));
    }
  }
  walk(dir); return result.sort();
}
function sameTrees(a, b) {
  const left = files(a); const right = files(b);
  return left.length === right.length && left.every((file, i) => file === right[i] && read(`${a}/${file}`) === read(`${b}/${file}`));
}
function executable(file) { return fs.existsSync(safe(file)) && (fs.statSync(safe(file)).mode & 0o111) !== 0; }

try {
  const major = Number(process.versions.node.split('.')[0]);
  check(major >= 18, `Node.js ${process.versions.node} (mínimo 18)`);
  validateBundle(); pass('bundle OKF v0.2 e manifesto válidos');

  const integration = JSON.parse(read('.specify/integration.json'));
  check(integration.default_integration === 'codex' && integration.installed_integrations?.includes('codex') &&
    integration.integration_settings?.codex?.parsed_options?.skills === true, 'Codex é a integração padrão em modo skills');

  const extension = document(read('.specify/extensions.yml')).toJS();
  check(extension.installed?.includes('okf') && extension.settings?.auto_execute_hooks === true, 'extensão OKF instalada com hooks automáticos');
  const requiredHooks = ['before_specify', 'after_specify', 'before_plan', 'before_tasks', 'after_tasks', 'before_implement', 'after_implement'];
  check(requiredHooks.every(name => Array.isArray(extension.hooks?.[name]) && extension.hooks[name].some(h => h.extension === 'okf' && h.enabled === true && h.optional === false)), 'todos os sete hooks OKF são obrigatórios e estão habilitados');
  check(sameTrees('.spec-kit/extensions/okf', '.specify/extensions/okf'), 'fonte e instalação da extensão OKF são idênticas');

  const templates = ['spec', 'plan', 'tasks'];
  check(templates.every(name => read(`.spec-kit/templates/${name}-template.md`) === read(`.specify/templates/overrides/${name}-template.md`)), 'templates-fonte e overrides canônicos são idênticos');

  const requiredSkills = ['constitution', 'specify', 'clarify', 'plan', 'tasks', 'analyze', 'implement', 'converge', 'checklist', 'taskstoissues',
    'okf-context', 'okf-stage', 'okf-activate', 'okf-sync', 'okf-archive'].map(name => `speckit-${name}`);
  let skillsValid = true;
  for (const name of requiredSkills) {
    const file = `.agents/skills/${name}/SKILL.md`;
    if (!fs.existsSync(safe(file))) { skillsValid = false; continue; }
    const match = read(file).match(/^---\r?\n([\s\S]*?)\r?\n---/);
    if (!match) { skillsValid = false; continue; }
    const data = document(match[1]).toJS();
    if (data.name !== name || typeof data.description !== 'string' || !data.description.trim()) skillsValid = false;
  }
  check(skillsValid, '15 skills possuem manifesto e nomes válidos');

  const scripts = ['new-spec.sh', 'fast-track.sh', 'feature.js', 'auto-sync-wiki.js', 'validate-okf.js', 'audit-workflow.js', 'install-hooks.sh'];
  check(scripts.every(name => executable(`.spec-kit/bin/${name}`)) && executable('.spec-kit/hooks/post-commit'), 'scripts e hook-fonte são executáveis');
  const installedHook = path.resolve(root, '.git/hooks/post-commit');
  check(fs.existsSync(installedHook) && (fs.statSync(installedHook).mode & 0o111) !== 0 && fs.readFileSync(installedHook, 'utf8') === read('.spec-kit/hooks/post-commit'), 'post-commit instalado e atualizado');

  check(fs.existsSync(safe('.spec-kit/node_modules/yaml')) && fs.existsSync(safe('.spec-kit/package-lock.json')), 'dependências locais estão instaladas e travadas');
  const active = fs.readdirSync(safe('specs/active'), { withFileTypes: true }).filter(e => e.isDirectory());
  check(active.length <= 1, 'há no máximo uma feature ativa');
  if (fs.existsSync(safe('.specify/feature.json'))) {
    const current = JSON.parse(read('.specify/feature.json')).feature_directory;
    check(typeof current === 'string' && fs.existsSync(safe(current)), 'feature.json aponta para uma feature existente');
  } else pass('nenhuma feature selecionada (estado inicial válido)');
  const lock = path.resolve(root, '.git/okf-auto-sync.lock');
  check(!fs.existsSync(lock), 'não existe trava residual de sincronização');
  try { require('node:child_process').execFileSync('git', ['rev-parse', '--verify', 'HEAD'], { cwd: root, stdio: 'ignore' }); }
  catch { warn('repositório ainda não possui commit-base; faça o commit inicial antes da primeira feature'); }
} catch (error) { fail(error.message); }

console.log(`\nAuditoria concluída: ${failures} falha(s).`);
process.exitCode = failures ? 1 : 0;
