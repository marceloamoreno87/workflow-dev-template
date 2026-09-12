#!/usr/bin/env node
const { validateBundle } = require('./lib');
try { validateBundle(); console.log('Bundle OKF v0.2 válido.'); }
catch (e) { console.error(e.message); process.exitCode = 1; }
