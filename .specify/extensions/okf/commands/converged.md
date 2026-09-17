---
description: Registrar o resultado de converge e sincronizar conhecimento somente quando convergido.
---

Leia o resultado efetivamente produzido por __SPECKIT_COMMAND_CONVERGE__ nesta
sessão e a feature atual. Este hook não substitui a avaliação semântica.
Se o resultado for tasks_appended, houver violação da constituição, falha de
qualidade ou tarefas executáveis pendentes, não registre convergência e não
altere a Wiki. Informe o próximo ciclo implement → verify → converge.

A única tarefa da Wiki é encerramento pós-convergência, não comportamento
executável faltante. Nunca a duplique nas fases de convergência.

Somente após resultado converged, registre em convergence-report.md os
requisitos/cenários/decisões/princípios avaliados, caminhos de código/testes,
resultados reais de qualidade e ausência de gaps. O relatório é artefato local
do hook, não uma alteração dos artefatos de intenção pelo core converge.
Confirme status verified (ou completed ao revalidar um ciclo já sincronizado).
Comite código e testes previstos antes de registrar a revisão, preservando
staged alheio. Execute:

`node .spec-kit/bin/convergence.js --reviewed-by codex/session`

Use a identidade real quando disponível. O recibo vincula código/testes,
spec/plano/tarefas e relatório, mas não prova correção semântica. Se o comando
falhar, pare sem sync. Depois execute __SPECKIT_COMMAND_OKF_SYNC__ na sessão
atual. Somente sync bem-sucedido marca a tarefa da Wiki e status completed.
