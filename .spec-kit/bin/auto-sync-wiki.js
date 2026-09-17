#!/usr/bin/env node
'use strict';
const { fs, path, root, git, safe, read, write, hash, document, metadata, setMetadata, feature, head, tasks, wikiTask, validateBundle, validateQuality, validateConvergence } = require('./lib');
const message = 'docs(wiki): atualiza conhecimento OKF v0.2 [auto-sync]';
function context(dir, commit) {
  const meta = metadata(`${dir}/spec.md`);
  if (!meta.base_commit) throw Error('Base ausente: ative a feature antes de implementar (feature.js activate).');
  if (!commit || !/^[0-9a-f]{40,64}$/.test(commit)) throw Error('Commit de implementação ausente/inválido.');
  git('merge-base', '--is-ancestor', commit, 'HEAD');
  const base = meta.base_commit === 'EMPTY' ? git('hash-object', '-t', 'tree', '--stdin') : meta.base_commit;
  if (meta.base_commit !== 'EMPTY') git('merge-base', '--is-ancestor', base, commit);
  if (git('status', '--porcelain', '--untracked-files=all', '--', 'src/').split('\n').some(l => l && !l.endsWith('.gitkeep'))) throw Error('Faça commit da implementação em src/ antes de revisar a Wiki.');
  if (git('diff', '--name-only', commit, 'HEAD', '--', 'src/')) throw Error('Código mudou depois da revisão; revise novamente.');
  const files = git('diff', '--no-renames', '--name-only', '-z', base, commit, '--', 'src/').split('\0').filter(f => f && path.basename(f) !== '.gitkeep');
  if (!files.length) throw Error('Nenhuma alteração de código no intervalo da feature.');
  const mappings = metadata(`${dir}/plan.md`, 'domains');
  if (!Array.isArray(mappings) || !mappings.length) throw Error('Domínios ausentes no plano.');
  const ids = new Set(); const paths = new Set();
  for (const d of mappings) {
    if (!d || !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(d.id) || !/^domains\/[a-z0-9/-]+\.md$/.test(d.path) || ['index.md', 'log.md'].includes(path.basename(d.path)) || ids.has(d.id) || paths.has(d.path)) throw Error('Domínio/caminho inválido ou duplicado no plano.');
    ids.add(d.id); paths.add(d.path); safe(`.knowledge/${d.path}`);
    if (!Array.isArray(d.depends_on) || !Array.isArray(d.source_paths) || !d.source_paths.length) throw Error(`Mapeamento inválido: ${d.id}`);
    for (const p of d.source_paths) { safe(p); if (!p.startsWith('src/') || /[*?]/.test(p)) throw Error(`source_paths inválido: ${p}`); }
  }
  const domains = mappings.filter(d => files.some(f => d.source_paths.some(p => p.endsWith('/') ? f.startsWith(p) : f === p)));
  for (const f of files) if (!domains.some(d => d.source_paths.some(p => p.endsWith('/') ? f.startsWith(p) : f === p))) throw Error(`Arquivo sem domínio: ${f}`);
  const wikiFiles = git('diff', '--no-renames', '--name-only', '-z', base, commit, '--', '.knowledge/').split('\0').filter(f => f && path.basename(f) !== '.gitkeep');
  return { spec: dir, base, commit, files, domains, wiki_files: wikiFiles,
    diff: git('diff', '--no-ext-diff', '--no-textconv', base, commit, '--', 'src/') };
}
function changedWiki() {
  return [...new Set([...git('diff', '--name-only', '-z', 'HEAD', '--', '.knowledge/').split('\0'),
    ...git('ls-files', '--others', '--exclude-standard', '-z', '--', '.knowledge/').split('\0')].filter(f => f && path.basename(f) !== '.gitkeep'))];
}
function ready(dir) {
  const meta = metadata(`${dir}/spec.md`);
  validateQuality(meta);
  tasks(dir, true);
  validateConvergence(dir);
}
function outputs(ctx, actor) {
  const result = new Map(); const now = new Date().toISOString();
  const manifestDoc = document(read('.knowledge/manifest.yaml'));
  const manifest = manifestDoc.toJS();
  const known = new Map((manifest.domains || []).map(d => [d.id, d]));
  for (const d of ctx.domains) {
    const { summary, ...mapping } = d;
    if (known.has(d.id) && known.get(d.id).path !== d.path) throw Error('Migração de caminho de domínio requer atualização explícita do manifesto.');
    known.set(d.id, { ...known.get(d.id), ...mapping });
  }
  const currentWiki = changedWiki();
  const reviewedWiki = new Set([...ctx.wiki_files, ...currentWiki]);
  const conceptFiles = new Set([...ctx.domains.map(d => `.knowledge/${d.path}`), ...reviewedWiki].filter(f => f.endsWith('.md') && !['index.md', 'log.md'].includes(path.basename(f))));
  for (const d of ctx.domains) if (!reviewedWiki.has(`.knowledge/${d.path}`)) throw Error(`O domínio afetado precisa de revisão semântica antes do recibo: ${d.id}.`);
  for (const file of conceptFiles) {
    if (!fs.existsSync(safe(file))) throw Error(`Preserve conceitos removidos como deprecated: ${file}`);
    const text = read(file); const fm = text.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
    if (!fm) throw Error(`Conceito sem frontmatter: ${file}`);
    const doc = document(fm[1]);
    if (typeof doc.get('type') !== 'string' || !doc.get('type').trim()) throw Error(`type inválido: ${file}`);
    doc.set('generated', { by: actor, at: now });
    // Uma mudança de conteúdo invalida verificações do conteúdo anterior.
    doc.delete('verified');
    const sources = doc.get('sources')?.toJSON() || [];
    if (!Array.isArray(sources)) throw Error(`sources inválido: ${file}`);
    const source = { id: 'implementation', resource: `git:${ctx.commit}`, title: 'Commit de implementação; git show <SHA>' };
    doc.set('sources', [...sources.filter(s => s.id !== 'implementation'), source]);
    result.set(file, `---\n${doc.toString()}---\n${text.slice(fm[0].length)}`);
  }
  const version = String(manifest.knowledge_version).match(/^(\d+)\.(\d+)\.(\d+)$/);
  if (!version) throw Error('knowledge_version inválida.');
  manifestDoc.set('domains', [...known.values()]);
  manifestDoc.set('knowledge_version', `${version[1]}.${version[2]}.${+version[3] + 1}`);
  manifestDoc.set('last_synced_commit', ctx.commit);
  result.set('.knowledge/manifest.yaml', manifestDoc.toString());
  const index = read('.knowledge/index.md');
  const block = /<!-- auto-sync:domains:start -->[\s\S]*?<!-- auto-sync:domains:end -->/;
  if (!block.test(index)) throw Error('Marcadores de índice ausentes.');
  result.set('.knowledge/index.md', index.replace(block, `<!-- auto-sync:domains:start -->\n${[...known.values()].map(d => `* [${d.id}](${d.path}) - conhecimento do domínio.`).join('\n')}\n<!-- auto-sync:domains:end -->`));
  for (const file of currentWiki) if (!result.has(file)) result.set(file, read(file));
  validateBundle(result);
  return result;
}
function main() {
  let explicit, commit, mode = 'publish', actor;
  const args = process.argv.slice(2);
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '--spec') explicit = args[++i];
    else if (a === '--commit') commit = args[++i];
    else if (a === '--prepare') mode = 'prepare';
    else if (a === '--reviewed-by') { mode = 'review'; actor = args[++i]; }
    else throw Error('Uso: auto-sync-wiki.js [--spec specs/.../nome] [--prepare | --reviewed-by codex/session] [--commit SHA]');
  }
  if (!head()) { if (mode === 'publish' && !explicit) { console.log('Sem commits; nada para sincronizar.'); return; } throw Error('Faça o commit de implementação antes do sync.'); }
  const dir = feature(explicit);
  if (mode === 'prepare') { fs.writeSync(1, JSON.stringify(context(dir, commit || head()), null, 2) + '\n'); return; }
  ready(dir);
  const lock = path.resolve(root, git('rev-parse', '--git-path', 'okf-auto-sync.lock'));
  let fd;
  try { fd = fs.openSync(lock, 'wx'); } catch (e) { if (e.code === 'EEXIST') throw Error(`Sync em andamento ou trava residual: ${lock}`); throw e; }
  try {
    fs.writeFileSync(fd, JSON.stringify({ pid: process.pid }));
    const receiptFile = `${dir}/wiki-sync.json`;
    if (mode === 'review') {
      if (!actor || !/^(?:human:|process:|[^\s/]+\/)[^\s]+$/.test(actor)) throw Error('Identifique quem revisou: codex/session ou human:nome.');
      const ctx = context(dir, commit || head());
      const beforeHead = head();
      if (fs.existsSync(safe(receiptFile))) {
        const previous = JSON.parse(read(receiptFile));
        if (previous.commit === ctx.commit && previous.base === ctx.base && previous.plan_hash === hash(read(`${dir}/plan.md`)) && previous.files && Object.keys(previous.files).length &&
            Object.entries(previous.files).every(([f, digest]) => f.startsWith('.knowledge/') && fs.existsSync(safe(f)) && hash(read(f)) === digest) &&
            changedWiki().every(f => previous.files[f])) {
          validateBundle();
          console.log('Revisão já registrada; execute a publicação.'); return;
        }
      }
      const data = outputs(ctx, actor);
      if (head() !== beforeHead) throw Error('HEAD mudou; revise novamente.');
      for (const [file, text] of data) write(file, text);
      write(receiptFile, JSON.stringify({ commit: ctx.commit, base: ctx.base, reviewed_by: actor,
        plan_hash: hash(read(`${dir}/plan.md`)), files: Object.fromEntries([...data].map(([f, text]) => [f, hash(text)])) }, null, 2) + '\n');
      const meta = metadata(`${dir}/spec.md`); meta.implementation_commit = ctx.commit; setMetadata(`${dir}/spec.md`, meta);
      console.log('Revisão registrada. Execute auto-sync-wiki.js para validar e comitar a Wiki.'); return;
    }
    const receipt = JSON.parse(read(receiptFile));
    if (commit && commit !== receipt.commit) throw Error('Revisão não corresponde ao commit do hook; execute $speckit-okf-sync.');
    const ctx = context(dir, receipt.commit);
    if (receipt.base !== ctx.base || receipt.plan_hash !== hash(read(`${dir}/plan.md`))) throw Error('Plano/base mudou após revisão.');
    if (!receipt.reviewed_by || !receipt.files || !Object.keys(receipt.files).length) throw Error('Recibo de revisão inválido.');
    for (const d of ctx.domains) if (!receipt.files[`.knowledge/${d.path}`]) throw Error(`Domínio não revisado: ${d.id}`);
    for (const [file, digest] of Object.entries(receipt.files)) {
      if (!file.startsWith('.knowledge/')) throw Error('Recibo contém caminho fora da Wiki.');
      if (hash(read(file)) !== digest) throw Error(`Wiki mudou após revisão: ${file}`);
    }
    const manifest = validateBundle();
    if (manifest.last_synced_commit !== receipt.commit) throw Error('Manifesto não corresponde à revisão.');
    const dirty = changedWiki();
    for (const file of dirty) if (!receipt.files[file]) throw Error(`Alteração da Wiki não revisada: ${file}`);
    if (dirty.length) {
      const before = head();
      git('add', '--intent-to-add', '--', ...dirty);
      if (head() !== before) throw Error('HEAD mudou antes do commit; repita a validação.');
      git('commit', '--only', '-m', message, '--', ...dirty);
    }
    const taskFile = `${dir}/tasks.md`;
    write(taskFile, read(taskFile).split('\n').map(line => line.includes(wikiTask) ? line.replace('- [ ]', '- [x]') : line).join('\n'));
    const meta = metadata(`${dir}/spec.md`); meta.status = 'completed'; setMetadata(`${dir}/spec.md`, meta);
    console.log(`Wiki sincronizada; HEAD ${head()}.`);
  } finally { fs.closeSync(fd); fs.unlinkSync(lock); }
}
try { main(); } catch (e) { console.error(`[wiki-sync] ${e.message}`); process.exitCode = 1; }
