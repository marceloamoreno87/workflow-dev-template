#!/usr/bin/env node
const { fs, path, safe, read, write, metadata, setMetadata, feature, head, git, tasks, hash, validateBundle, validateQuality } = require('./lib');
try {
  const [action, explicit] = process.argv.slice(2);
  if (!['stage', 'activate', 'select', 'archive'].includes(action)) throw Error('Uso: feature.js stage|activate|select|archive [specs/.../nome]');
  let dir = feature(explicit);
  const meta = metadata(`${dir}/spec.md`);
  let target = dir;
  if (action === 'stage') {
    if (dir.startsWith('specs/active/') || dir.startsWith('specs/archive/')) throw Error('Não é permitido recolocar feature ativa/arquivada em backlog.');
    target = `specs/backlog/${path.basename(dir)}`;
  }
  if (action === 'activate') {
    if (dir.startsWith('specs/archive/')) throw Error('Feature arquivada; crie novo ciclo.');
    if (!['approved', 'completed'].includes(meta.status) || !/^human:[^\s]+$/.test(meta.approved_by || '')) throw Error('Spec precisa de aprovação explícita registrada como human:<id>.');
    const domains = metadata(`${dir}/plan.md`, 'domains');
    const manifest = validateBundle();
    if (!Array.isArray(domains) || !domains.length) throw Error('Preencha os domínios OKF no plano.');
    const existingIds = new Map(manifest.domains.map(d => [d.id, d.path]));
    const existingPaths = new Map(manifest.domains.map(d => [d.path, d.id]));
    const mappedIds = new Set(); const mappedPaths = new Set();
    for (const d of domains) {
      if (!d || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(d.id || '') || !/^domains\/[a-z0-9/-]+\.md$/.test(d.path || '') ||
          !Array.isArray(d.depends_on) || !Array.isArray(d.source_paths) || !d.source_paths.length ||
          d.source_paths.some(p => typeof p !== 'string' || !p.startsWith('src/') || /[*?]/.test(p))) throw Error(`Mapeamento OKF inválido no plano: ${d?.id || 'sem id'}.`);
      if (mappedIds.has(d.id) || mappedPaths.has(d.path) || (existingIds.has(d.id) && existingIds.get(d.id) !== d.path) ||
          (existingPaths.has(d.path) && existingPaths.get(d.path) !== d.id)) throw Error(`ID/caminho OKF duplicado ou incompatível: ${d.id}.`);
      mappedIds.add(d.id); mappedPaths.add(d.path);
      safe(`.knowledge/${d.path}`); for (const p of d.source_paths) safe(p);
    }
    const availableIds = new Set([...existingIds.keys(), ...mappedIds]);
    for (const d of domains) if (d.depends_on.some(dep => dep === d.id || !availableIds.has(dep))) throw Error(`Dependência OKF inválida no plano: ${d.id}.`);
    tasks(dir);
    target = `specs/active/${path.basename(dir)}`;
    const otherActive = fs.readdirSync(safe('specs/active'), { withFileTypes: true })
      .filter(e => e.isDirectory() && e.name !== path.basename(dir));
    if (otherActive.length) throw Error(`Outra feature já está ativa: ${otherActive.map(e => e.name).join(', ')}.`);
    if (!Object.hasOwn(meta, 'base_commit') || meta.base_commit === null) {
      if (git('status', '--porcelain', '--untracked-files=all', '--', 'src/').split('\n').some(l => l && !l.endsWith('.gitkeep'))) throw Error('Commit das alterações anteriores em src/ necessário antes de ativar.');
      meta.base_commit = head() || 'EMPTY';
    }
    if (git('status', '--porcelain', '--untracked-files=all', '--', '.knowledge/')) throw Error('A Wiki precisa estar limpa antes de ativar ou retomar a feature.');
  }
  if (action === 'archive') {
    validateBundle();
    validateQuality(meta);
    if (tasks(dir, true).some(t => t[1] === ' ')) throw Error('Spec/testes/tarefas ainda não concluídos.');
    const receipt = JSON.parse(read(`${dir}/wiki-sync.json`));
    const manifest = validateBundle();
    if (git('status', '--porcelain', '--untracked-files=all', '--', 'src/')) throw Error('Código possui alterações locais; conclua e sincronize antes de arquivar.');
    if (receipt.plan_hash !== hash(read(`${dir}/plan.md`))) throw Error('Plano mudou desde a revisão; sincronize novamente.');
    if (meta.implementation_commit !== receipt.commit) throw Error('Spec não corresponde ao recibo de revisão.');
    if (manifest.last_synced_commit !== receipt.commit || git('diff', receipt.commit, 'HEAD', '--', 'src/')) throw Error('Wiki não corresponde à implementação atual.');
    for (const [file, digest] of Object.entries(receipt.files)) if (hash(read(file)) !== digest) throw Error(`Wiki mudou desde revisão: ${file}`);
    if (git('status', '--porcelain', '--untracked-files=all', '--', '.knowledge/')) throw Error('Wiki precisa estar commitada antes de arquivar.');
    target = `specs/archive/${path.basename(dir)}`;
  }
  if (target !== dir) {
    if (fs.existsSync(safe(target))) throw Error(`Destino já existe: ${target}`);
    fs.mkdirSync(path.dirname(safe(target)), { recursive: true });
    fs.renameSync(safe(dir), safe(target));
    // O fast-track legado expõe um link ao lado da pasta; remova só esse link.
    const shortcut = path.join(path.dirname(safe(dir)), `${path.basename(dir)}.md`);
    if (fs.existsSync(path.dirname(shortcut))) {
      try { if (fs.lstatSync(shortcut).isSymbolicLink() && fs.readlinkSync(shortcut) === `${path.basename(dir)}/spec.md`) fs.unlinkSync(shortcut); }
      catch (e) { if (e.code !== 'ENOENT') throw e; }
    }
  }
  if (action === 'activate') setMetadata(`${target}/spec.md`, meta);
  write('.specify/feature.json', JSON.stringify({ feature_directory: target }, null, 2) + '\n');
  if (action === 'archive') {
    const trackedOld = git('ls-files', '--', dir);
    const commitPaths = trackedOld ? [dir, target] : [target];
    git('add', '--intent-to-add', '--', target);
    if (git('status', '--porcelain', '--untracked-files=all', '--', ...commitPaths)) {
      try { git('commit', '--only', '-m', `docs(spec): arquiva ${path.basename(target)}`, '--', ...commitPaths); }
      catch { throw Error(`Feature movida para ${target}, mas o commit de arquivo falhou. Corrija o Git e execute archive novamente.`); }
    }
  }
  console.log(JSON.stringify({ FEATURE_DIR: safe(target), feature_directory: target }));
} catch (e) { console.error(e.message); process.exitCode = 1; }
