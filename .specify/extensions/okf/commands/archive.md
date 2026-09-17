---
description: Arquivar a feature concluída e manter o contexto oficial do Spec Kit.
---

Leia `.spec-kit/system-rules.md` e a feature atual. Verifique aprovação,
evidências de testes/lint/tipos e que todos os itens de tasks.md estão concluídos.
Confirme um convergence.json válido; se ausente ou obsoleto, execute
__SPECKIT_COMMAND_CONVERGE__ antes de arquivar. Se houver trabalho
restante, mantenha active e informe as tarefas; não declare convergência.

Se houver mudanças de código ou conhecimento ainda não sincronizadas, execute
__SPECKIT_COMMAND_OKF_SYNC__ e confirme o commit. Depois execute
`node .spec-kit/bin/feature.js archive`. O comando verifica o recibo de Wiki,
move a pasta, atualiza `.specify/feature.json` e cria um commit local exclusivo
dos artefatos da spec. Não sobrescreva arquivo histórico e não faça push.
