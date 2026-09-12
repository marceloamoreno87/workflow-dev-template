---
name: speckit-okf-context
description: Consultar contexto OKF antes de especificar, planejar ou gerar tarefas.
compatibility: Requires spec-kit project structure with .specify/ directory
metadata:
  author: github-spec-kit
  source: okf:commands/context.md
---

Leia `.spec-kit/system-rules.md`, `.knowledge/manifest.yaml` e
`.knowledge/index.md`. Consulte os conceitos dos domínios afetados, dependências,
contratos e ADRs pertinentes. Registre lacunas sem inventar decisões.

Na etapa specify, prossiga com a criação oficial da spec; o hook stage cuidará
de backlog. Nas etapas plan/tasks, leia `.specify/feature.json` e o spec.md
correspondente. Registre aprovação somente se o desenvolvedor já a deu na
conversa. Sem aprovação, apresente a spec pronta e peça sua aprovação; não
preencha approved_by por conta própria. Considere "aprovado, gere o plano"
aprovação suficiente para a spec atual, sem perguntar novamente.

Preserve os blocos JSON dos templates. Testes são obrigatórios pela constituição.