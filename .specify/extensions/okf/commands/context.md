---
description: Consultar contexto OKF antes de especificar, planejar ou gerar tarefas.
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


Descubra conhecimento seletivamente: índice → conceitos diretamente afetados →
dependências/links relevantes → contratos/ADRs. Comece com cerca de 5–10
conceitos quando suficiente; amplie por necessidade justificada, sem carregar
o bundle inteiro. Avalie status, verified, stale_after e sources; conhecimento
obsoleto, sem fonte ou contraditório requer descoberta dirigida e uma lacuna
explícita, nunca uma arquitetura presumida. OKF descreve o estado existente;
spec descreve intenção; constituição governa; código/testes evidenciam realidade.

No plan, preencha Knowledge Context com caminhos/fontes, validade e restrições
observadas, distinguindo hipóteses. Vincule cada decisão aos conceitos e aos
requisitos. Contratos específicos continuam em OpenAPI/Protobuf ou equivalente.
No specify, preserve what/why e deixe escolhas técnicas para plan.

Antes de converge, trate a única tarefa da Wiki como encerramento diferido:
ela não é gap de implementação nem exige nova tarefa. Todo trabalho executável
é avaliado normalmente. O hook after_converge só fará sync se o resultado real
for converged; tarefas acrescentadas mantêm a Wiki no estado anterior.
