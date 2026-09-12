const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const YAML = require('yaml');
const { execFileSync, spawnSync } = require('node:child_process');
const source = path.resolve(__dirname, '../..');
const wikiTask = 'Atualizar a LLM Wiki em .knowledge/ com as mudanças realizadas neste ciclo';
const quality = { tests: { command: 'node --test', passed: true }, coverage: { command: 'node --test --experimental-test-coverage', percent: 90, passed: true }, lint: { command: 'npm run lint', passed: true }, types: { command: 'npm run typecheck', passed: true } };
function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'okf-test-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  for (const dir of ['.spec-kit', '.specify', '.knowledge', 'specs', 'src']) fs.cpSync(path.join(source, dir), path.join(root, dir), { recursive: true });
  const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  git('init', '-q'); git('config', 'user.name', 'OKF Test'); git('config', 'user.email', 'okf@example.invalid');
  git('config', 'commit.gpgsign', 'false'); git('config', 'core.hooksPath', path.join(root, 'disabled-hooks'));
  const read = file => fs.readFileSync(path.join(root, file), 'utf8');
  const write = (file, text) => { fs.mkdirSync(path.dirname(path.join(root, file)), { recursive: true }); fs.writeFileSync(path.join(root, file), text); };
  write('.gitignore', '.spec-kit/node_modules/\n');
  const run = (file, ...args) => spawnSync(file.endsWith('.js') ? process.execPath : 'bash', [path.join(root, '.spec-kit/bin', file), ...args], { cwd: root, encoding: 'utf8' });
  const meta = (file, data, kind = 'metadata') => write(file, `<!-- spec-kit:${kind} -->\n\x60\x60\x60json\n${JSON.stringify(data)}\n\x60\x60\x60\n`);
  return { root, git, read, write, run, meta };
}
function ok(result) { assert.equal(result.status, 0, `${result.stderr}\n${result.stdout}`); return result.stdout; }
function setup(f) {
  f.git('add', '.'); f.git('commit', '-qm', 'chore: scaffold');
  ok(f.run('new-spec.sh', 'sample'));
  f.meta('specs/backlog/sample/spec.md', { status: 'approved', approved_by: 'human:test', base_commit: null, tests_passed: false });
  f.meta('specs/backlog/sample/plan.md', [{ id: 'sample', path: 'domains/sample.md', depends_on: [], source_paths: ['src/'] }], 'domains');
  f.write('specs/backlog/sample/tasks.md', `- [ ] T001 [US1] Testes e implementação\n- [ ] T002 ${wikiTask}\n`);
  ok(f.run('feature.js', 'activate'));
  return f.git('rev-parse', 'HEAD');
}
function implementation(f) {
  const base = setup(f);
  f.write('src/sample.js', 'module.exports = () => 42;\n'); f.git('add', 'src'); f.git('commit', '-qm', 'feat: sample');
  const commit = f.git('rev-parse', 'HEAD');
  f.meta('specs/active/sample/spec.md', { status: 'completed', approved_by: 'human:test', base_commit: base, implementation_commit: commit, tests_passed: true, quality });
  f.write('specs/active/sample/tasks.md', `- [x] T001 [US1] Testes e implementação\n- [ ] T002 ${wikiTask}\n`);
  return { base, commit };
}
function review(f) {
  f.write('.knowledge/domains/sample.md', '---\ntype: Domain\ntitle: Sample\nstatus: draft\ncustom_field: keep\nsources:\n  - resource: https://example.invalid/design\n---\n# Sample\n\nRetorna 42; contrato revisado a partir do código.\n');
  ok(f.run('auto-sync-wiki.js', '--reviewed-by', 'codex/test'));
}

test('integração oficial, skills e hooks OKF estão registrados', () => {
  const config = JSON.parse(fs.readFileSync(path.join(source, '.specify/integration.json')));
  assert.ok(config.installed_integrations.includes('codex'));
  const hooks = YAML.parse(fs.readFileSync(path.join(source, '.specify/extensions.yml'), 'utf8'));
  for (const hook of Object.values(hooks.hooks).flat()) {
    assert.equal(hook.optional, false);
    const skill = path.join(source, '.agents/skills', hook.command.replaceAll('.', '-'), 'SKILL.md');
    assert.ok(fs.existsSync(skill), skill);
  }
  for (const name of ['specify', 'plan', 'tasks', 'implement', 'converge']) assert.ok(fs.existsSync(path.join(source, `.agents/skills/speckit-${name}/SKILL.md`)));
});

test('criação seleciona feature; scripts oficiais resolvem overrides e rejeitam colisão', t => {
  const f = fixture(t);
  assert.equal(f.run('new-spec.sh', '../escape').status, 2);
  ok(f.run('new-spec.sh', 'sample')); assert.notEqual(f.run('new-spec.sh', 'sample').status, 0);
  assert.equal(JSON.parse(f.read('.specify/feature.json')).feature_directory, 'specs/backlog/sample');
  const result = spawnSync('bash', ['.specify/scripts/bash/setup-tasks.sh', '--json'], { cwd: f.root, encoding: 'utf8' });
  const data = JSON.parse(ok(result));
  assert.ok(data.TASKS_TEMPLATE.includes('.specify/templates/overrides/'));
  assert.ok(data.TASKS_TEMPLATE_CONTENT.includes(wikiTask));
  assert.notEqual(f.run('feature.js', 'activate').status, 0);
});

test('stage e activate preservam contexto e base original', t => {
  const f = fixture(t);
  f.meta('specs/001-example/spec.md', { status: 'draft' });
  ok(f.run('feature.js', 'stage', 'specs/001-example'));
  assert.ok(fs.existsSync(path.join(f.root, 'specs/backlog/001-example/spec.md')));
  const base = setup(f);
  ok(f.run('feature.js', 'activate'));
  assert.match(f.read('specs/active/sample/spec.md'), new RegExp(base));
  assert.equal(JSON.parse(f.read('.specify/feature.json')).feature_directory, 'specs/active/sample');
});

test('ativação bloqueia Wiki suja, mapeamento inválido e outra feature ativa', t => {
  const f = fixture(t); f.git('add', '.'); f.git('commit', '-qm', 'chore: scaffold');
  ok(f.run('new-spec.sh', 'one'));
  f.meta('specs/backlog/one/spec.md', { status: 'approved', approved_by: 'human:test', base_commit: null });
  f.meta('specs/backlog/one/plan.md', [{ id: 'one', path: 'domains/one.md', depends_on: [], source_paths: ['src/*'] }], 'domains');
  f.write('specs/backlog/one/tasks.md', `- [ ] T001 Teste\n- [ ] T002 ${wikiTask}\n`);
  assert.notEqual(f.run('feature.js', 'activate').status, 0);
  f.meta('specs/backlog/one/plan.md', [{ id: 'one', path: 'domains/one.md', depends_on: [], source_paths: ['src/'] }], 'domains');
  f.write('.knowledge/index.md', f.read('.knowledge/index.md') + '\nlocal\n');
  assert.notEqual(f.run('feature.js', 'activate').status, 0);
  f.write('.knowledge/index.md', f.git('show', 'HEAD:.knowledge/index.md') + '\n');
  ok(f.run('feature.js', 'activate'));
  f.write('specs/backlog/two/spec.md', '<!-- spec-kit:metadata -->\n```json\n{"status":"approved","approved_by":"human:test","base_commit":null}\n```\n');
  f.meta('specs/backlog/two/plan.md', [{ id: 'two', path: 'domains/two.md', depends_on: [], source_paths: ['src/'] }], 'domains');
  f.write('specs/backlog/two/tasks.md', `- [ ] T001 Teste\n- [ ] T002 ${wikiTask}\n`);
  assert.notEqual(f.run('feature.js', 'activate', 'specs/backlog/two').status, 0);
});

test('diff inclui múltiplos commits; publicar exige revisão semântica registrada', t => {
  const f = fixture(t); implementation(f);
  f.write('src/second.js', 'module.exports = 1;\n'); f.git('add', 'src'); f.git('commit', '-qm', 'feat: second');
  const data = JSON.parse(ok(f.run('auto-sync-wiki.js', '--prepare')));
  assert.deepEqual(data.files.sort(), ['src/sample.js', 'src/second.js']);
  assert.match(data.diff, /42/); assert.match(data.diff, /second.js/);
  assert.notEqual(f.run('auto-sync-wiki.js').status, 0);
});

test('revisão atualiza OKF, preserva staged externo e arquiva com contexto correto', t => {
  const f = fixture(t); const { commit } = implementation(f); review(f);
  const data = YAML.parse(f.read('.knowledge/domains/sample.md').split('---')[1]);
  assert.equal(data.custom_field, 'keep'); assert.equal(data.generated.by, 'codex/test');
  assert.equal(data.sources.length, 2); assert.equal(data.sources[1].resource, `git:${commit}`);
  f.write('unrelated.txt', 'staged\n'); f.git('add', 'unrelated.txt');
  ok(f.run('auto-sync-wiki.js'));
  assert.equal(f.git('diff', '--cached', '--name-only'), 'unrelated.txt');
  assert.ok(f.git('diff-tree', '--no-commit-id', '--name-only', '-r', 'HEAD').split('\n').every(p => p.startsWith('.knowledge/')));
  assert.ok(f.read('specs/active/sample/tasks.md').includes(`- [x] T002 ${wikiTask}`));
  const head = f.git('rev-parse', 'HEAD'); ok(f.run('auto-sync-wiki.js')); assert.equal(f.git('rev-parse', 'HEAD'), head);
  ok(f.run('feature.js', 'archive'));
  assert.equal(JSON.parse(f.read('.specify/feature.json')).feature_directory, 'specs/archive/sample');
  assert.match(f.git('log', '-1', '--format=%s'), /^docs\(spec\): arquiva sample$/);
  assert.equal(f.git('diff', '--cached', '--name-only'), 'unrelated.txt');
});

test('YAML inválido ou type vazio bloqueia validação; Wiki modificada invalida recibo', t => {
  const f = fixture(t); implementation(f); review(f);
  const file = '.knowledge/domains/sample.md'; const original = f.read(file);
  f.write(file, '---\ntype: [\n---\nInvalid\n'); assert.notEqual(f.run('validate-okf.js').status, 0);
  f.write(file, '---\ntype: ""\n---\nInvalid\n'); assert.notEqual(f.run('validate-okf.js').status, 0);
  f.write(file, original + '\nMudança após revisão\n'); assert.notEqual(f.run('auto-sync-wiki.js').status, 0);
});

test('sync exige evidências estruturadas e revisão real de todo domínio afetado', t => {
  const f = fixture(t); const { base, commit } = implementation(f);
  f.meta('specs/active/sample/spec.md', { status: 'completed', approved_by: 'human:test', base_commit: base, implementation_commit: commit, tests_passed: true });
  assert.notEqual(f.run('auto-sync-wiki.js', '--reviewed-by', 'codex/test').status, 0);
  f.meta('specs/active/sample/spec.md', { status: 'completed', approved_by: 'human:test', base_commit: base, implementation_commit: commit, tests_passed: true, quality });
  assert.notEqual(f.run('auto-sync-wiki.js', '--reviewed-by', 'codex/test').status, 0);
});

test('alteração semântica remove verified obsoleto', t => {
  const f = fixture(t); implementation(f);
  f.write('.knowledge/domains/sample.md', '---\ntype: Domain\ntitle: Sample\nverified: {by: human:old, at: 2026-01-01T00:00:00Z}\n---\n# Sample\n\nNovo comportamento.\n');
  ok(f.run('auto-sync-wiki.js', '--reviewed-by', 'codex/test'));
  const data = YAML.parse(f.read('.knowledge/domains/sample.md').split('---')[1]);
  assert.equal(data.verified, undefined);
});

test('código novo após revisão invalida recibo; trava impede concorrência', t => {
  const f = fixture(t); implementation(f); review(f);
  f.write('src/sample.js', 'module.exports = () => 43;\n'); f.git('add', 'src'); f.git('commit', '-qm', 'fix: new code');
  assert.notEqual(f.run('auto-sync-wiki.js').status, 0);
  f.write('.git/okf-auto-sync.lock', 'held'); assert.notEqual(f.run('auto-sync-wiki.js', '--reviewed-by', 'codex/test').status, 0);
});

test('falha de commit pode ser retomada sem outro bump de versão', t => {
  const f = fixture(t); implementation(f); review(f);
  const before = YAML.parse(f.read('.knowledge/manifest.yaml')).knowledge_version;
  ok(f.run('auto-sync-wiki.js', '--reviewed-by', 'codex/test'));
  assert.equal(YAML.parse(f.read('.knowledge/manifest.yaml')).knowledge_version, before);
  f.write('disabled-hooks/pre-commit', '#!/bin/sh\nexit 1\n'); fs.chmodSync(path.join(f.root, 'disabled-hooks/pre-commit'), 0o755);
  assert.equal(f.run('auto-sync-wiki.js').status, 1);
  fs.unlinkSync(path.join(f.root, 'disabled-hooks/pre-commit'));
  ok(f.run('auto-sync-wiki.js'));
  assert.equal(YAML.parse(f.read('.knowledge/manifest.yaml')).knowledge_version, before);
});

test('arquivamento recusa código local ainda não revisado', t => {
  const f = fixture(t); implementation(f); review(f); ok(f.run('auto-sync-wiki.js'));
  f.write('src/sample.js', 'module.exports = () => 99;\n');
  assert.notEqual(f.run('feature.js', 'archive').status, 0);
  assert.equal(JSON.parse(f.read('.specify/feature.json')).feature_directory, 'specs/active/sample');
});

test('fast-track mantém contexto; instalador preserva hooks existentes', t => {
  const f = fixture(t); ok(f.run('fast-track.sh', 'fix'));
  assert.equal(JSON.parse(f.read('.specify/feature.json')).feature_directory, 'specs/active/fast-track-fix');
  assert.notEqual(f.run('fast-track.sh', 'second').status, 0);
  assert.notEqual(f.run('feature.js', 'activate').status, 0);
  f.git('config', '--unset', 'core.hooksPath');
  f.write('.git/hooks/post-commit', '#!/bin/sh\n# usuário\n');
  assert.notEqual(f.run('install-hooks.sh').status, 0);
  fs.unlinkSync(path.join(f.root, '.git/hooks/post-commit'));
  ok(f.run('install-hooks.sh')); ok(f.run('install-hooks.sh'));
  f.write('.git/hooks/post-commit', '#!/usr/bin/env bash\n# spec-kit OKF post-commit v1\nexit 9\n');
  ok(f.run('install-hooks.sh'));
  assert.equal(f.read('.git/hooks/post-commit'), f.read('.spec-kit/hooks/post-commit'));
});

test('hook publica revisão preparada em segundo plano sem loop', async t => {
  const f = fixture(t); const { commit } = implementation(f); review(f);
  f.git('config', '--unset', 'core.hooksPath'); ok(f.run('install-hooks.sh'));
  ok(spawnSync('bash', ['.git/hooks/post-commit'], { cwd: f.root, encoding: 'utf8' }));
  const log = path.join(f.root, '.git/okf-auto-sync.log');
  for (let i = 0; i < 200; i++) {
    if (fs.existsSync(log) && fs.readFileSync(log, 'utf8').includes('Wiki sincronizada')) break;
    await new Promise(resolve => setTimeout(resolve, 25));
  }
  assert.match(fs.readFileSync(log, 'utf8'), /Wiki sincronizada/);
  assert.equal(f.git('rev-parse', 'HEAD~1'), commit);
  const head = f.git('rev-parse', 'HEAD');
  ok(spawnSync('bash', ['.git/hooks/post-commit'], { cwd: f.root, encoding: 'utf8' }));
  assert.equal(f.git('rev-parse', 'HEAD'), head);
});
