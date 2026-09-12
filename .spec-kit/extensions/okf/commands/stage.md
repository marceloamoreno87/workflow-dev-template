---
description: Colocar a spec recém-criada no backlog sem perder o contexto do Spec Kit.
---

Execute `node .spec-kit/bin/feature.js stage`. O comando lê a feature atual,
move a pasta para `specs/backlog/` e atualiza `.specify/feature.json`. Se já
existir uma feature ativa, ela permanece como contexto; a nova spec apenas fica
na fila. Informe o caminho retornado ao concluir. A criação não significa aprovação.
