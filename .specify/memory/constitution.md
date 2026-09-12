# Constituição de workflow-dev

## I. Spec antes de código

Mudanças em src/ exigem spec aprovada em specs/active/, com plan.md e tasks.md.
A aprovação do desenvolvedor deve ser registrada; criar uma spec não a aprova.

## II. Conhecimento antes de decisões

Consulte .knowledge/manifest.yaml, .knowledge/index.md e conceitos pertinentes.
Preserve a Wiki OKF v0.2; não transforme suposições em fatos.

## III. Testes primeiro

TDD é obrigatório. Testes devem falhar pelo motivo esperado antes do código e
passar depois. A política inicial exige cobertura mínima de 80%, cenários
críticos cobertos, lint sem avisos e tipos estritos quando aplicáveis à stack.
Defina e execute os comandos concretos no plano; registre evidências.

## IV. Escopo e contexto

Limite implementação aos arquivos mapeados. Use o contexto oficial em
.specify/feature.json. Mova specs com .spec-kit/bin/feature.js.

## V. Conhecimento atualizado ao concluir

Toda lista de tarefas inclui atualização da Wiki. Execute a revisão semântica
com $speckit-okf-sync antes de declarar implementação concluída. Os hooks da
extensão OKF conectam o fluxo oficial a essa tarefa. O script valida YAML,
proveniência e recibo e comita somente a Wiki. Arquive após convergência.

## Governança

As regras detalhadas estão em .spec-kit/system-rules.md. Alterações destes
princípios exigem decisão explícita do desenvolvedor. A stack da aplicação será
definida na primeira feature; Node.js é ferramenta deste workflow.

**Version**: 1.0.0 | **Ratified**: 2026-09-12 | **Last Amended**: 2026-09-12
