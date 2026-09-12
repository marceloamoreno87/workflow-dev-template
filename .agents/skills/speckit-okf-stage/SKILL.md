---
name: speckit-okf-stage
description: Colocar a spec recém-criada no backlog sem perder o contexto do Spec Kit.
compatibility: Requires spec-kit project structure with .specify/ directory
metadata:
  author: github-spec-kit
  source: okf:commands/stage.md
---

Execute `node .spec-kit/bin/feature.js stage`. O comando lê a feature atual,
move a pasta para `specs/backlog/` e atualiza `.specify/feature.json`.
Não substitua o contexto manualmente pelo caminho anterior. Informe o caminho
retornado ao concluir a etapa specify. A criação não significa aprovação.