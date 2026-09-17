---
name: speckit-okf-sync
description: Atualizar a Wiki OKF com análise semântica da diff completa da feature.
compatibility: Requires spec-kit project structure with .specify/ directory
metadata:
  author: github-spec-kit
  source: okf:commands/sync.md
---

Antes de editar conhecimento, execute
`node .spec-kit/bin/convergence.js --check` para validar convergence.json com
a implementação e artefatos atuais; sem convergência comprovada, execute
$speckit-converge e não sincronize se houver gaps.

Leia `.spec-kit/templates/wiki-sync-prompt.md` e execute seu procedimento na
feature atual de `.specify/feature.json`. Você é o sintetizador: leia a diff,
o código pertinente e os conceitos, e edite o conhecimento durável.
Não substitua essa revisão por uma cópia do summary do plano. Edite cada domínio
afetado e registre evidências estruturadas de testes, cobertura, lint e tipos.

Finalize com `node .spec-kit/bin/auto-sync-wiki.js`. Esse comando valida a revisão
e o YAML, atualiza proveniência/índice e faz o commit exclusivo da Wiki. O hook
Git não invoca outra sessão de IA; ele só publica uma revisão já preparada.
Informe o SHA e eventuais pendências. Não diga concluído se faltar evidência.