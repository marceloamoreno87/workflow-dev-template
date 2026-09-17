---
description: Validar e ativar uma spec antes da implementação.
---

Leia `.spec-kit/system-rules.md` e a feature de `.specify/feature.json`.
Confirme que aprovação explícita está registrada e que spec, plano e tarefas
estão preenchidos, incluindo arquivos permitidos, comandos de qualidade e
domínios OKF. Se a aprovação já consta da conversa, registre-a sem perguntar
novamente. Não invente aprovação nem resultados de testes.

Execute `node .spec-kit/bin/feature.js activate`. O comando move a pasta para
active, atualiza o contexto oficial e registra o commit base para cobrir todas
as mudanças da feature. Se já estiver ativa, preserva a base original. Uma
segunda feature ativa ou mudanças locais em `src/`/`.knowledge/` bloqueiam a
ativação; faça commits separados ou conclua o ciclo atual.

Antes de implementar, leia os conceitos afetados e verifique que a checklist
tem testes antes de código. Respeite fases/IDs solicitados. Durante implement, não execute a tarefa Wiki,
mesmo quando todas as tarefas de código tiverem terminado. A tarefa da Wiki só é executada após converge; não a
marque até esse hook concluir. Falha de ativação bloqueia alterações em src/.
