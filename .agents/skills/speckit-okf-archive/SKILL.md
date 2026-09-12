---
name: speckit-okf-archive
description: Arquivar a feature concluída e manter o contexto oficial do Spec Kit.
compatibility: Requires spec-kit project structure with .specify/ directory
metadata:
  author: github-spec-kit
  source: okf:commands/archive.md
---

Leia `.spec-kit/system-rules.md` e a feature atual. Verifique aprovação,
evidências de testes/lint/tipos e que todos os itens de tasks.md estão concluídos.
Execute $speckit-converge antes de arquivar. Se houver trabalho
restante, mantenha active e informe as tarefas; não declare convergência.

Se houver mudanças de código ou conhecimento ainda não sincronizadas, execute
$speckit-okf-sync e confirme o commit. Depois execute
`node .spec-kit/bin/feature.js archive`. O comando verifica o recibo de Wiki,
move a pasta, atualiza `.specify/feature.json` e cria um commit local exclusivo
dos artefatos da spec. Não sobrescreva arquivo histórico e não faça push.