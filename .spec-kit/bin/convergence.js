#!/usr/bin/env node
'use strict';
const { write, feature, head, convergenceState, validateConvergence } = require('./lib');
try {
  let actor, explicit, check = false;
  const args = process.argv.slice(2);
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--reviewed-by') actor = args[++i];
    else if (args[i] === '--check') check = true;
    else if (args[i] === '--spec') explicit = args[++i];
    else throw Error('Uso: convergence.js --check | --reviewed-by codex/session [--spec specs/active/nome]');
  }
  const dir = feature(explicit);
  if (check) { validateConvergence(dir); console.log('Convergência válida.'); process.exit(0); }
  if (!/^(?:human:|process:|[^\s/]+\/)[^\s]+$/.test(actor || '')) throw Error('Identifique quem efetivamente avaliou a convergência.');
  if (!dir.startsWith('specs/active/')) throw Error('Convergência só pode ser registrada para a feature ativa.');
  const state = convergenceState(dir);
  if (!state.base || !head()) throw Error('Ative e comite a implementação antes de registrar convergência.');
  write(`${dir}/convergence.json`, JSON.stringify({ outcome: 'converged', reviewed_by: actor,
    at: new Date().toISOString(), commit: head(), ...state }, null, 2) + '\n');
  console.log('Convergência registrada; a Wiki pode ser revisada. O recibo não substitui a avaliação do código.');
} catch (error) { console.error(`[convergence] ${error.message}`); process.exitCode = 1; }
