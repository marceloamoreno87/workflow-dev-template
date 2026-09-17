# Diretrizes para agentes de system design

Estas diretrizes reduzem erros comuns na criação e avaliação de arquiteturas de software, desde sistemas simples até soluções distribuídas avançadas. Aplique-as junto às instruções específicas do projeto; em caso de conflito, as instruções mais específicas prevalecem.

Elas privilegiam clareza, precisão e decisões justificadas em vez de complexidade ou velocidade. Para problemas pequenos e inequívocos, use bom senso: não transforme um design simples em um processo burocrático nem em uma arquitetura distribuída sem necessidade.

## 0. Escopo operacional: design, não produção

Todo trabalho deve resultar, por padrão, em documentação, diagramas, estimativas, contratos e exemplos locais.

- Não crie, altere ou execute infraestrutura, pipelines de CI/CD, deploy, publicação, provisionamento ou automações externas.
- Não trate uma proposta arquitetural como pronta para produção sem informações suficientes sobre ambiente, operação, segurança, custos, restrições organizacionais e critérios de validação.
- Só trabalhe em infraestrutura ou produção quando isso for pedido explicitamente e houver contexto completo, credenciais/permissões necessárias e critérios de sucesso.
- Quando o pedido for apenas de design, limite código a protótipos, contratos ou exemplos. Para implementação solicitada, aplique o workflow Spec Kit da seção 6.

## 1. Entenda o problema antes de desenhar

Não presuma requisitos, escala ou restrições, nem esconda incertezas.

Antes de propor a arquitetura:

- Identifique os objetivos de negócio, os atores e os fluxos principais do sistema.
- Separe requisitos funcionais dos não funcionais, como disponibilidade, latência, consistência, durabilidade, segurança, privacidade, custo e observabilidade.
- Confirme o escopo e explicite o que fica fora dele.
- Levante restrições relevantes, como tecnologias existentes, integrações, orçamento, prazo, equipe, região, compliance e ambiente operacional.
- Estime escala quando ela influenciar decisões: usuários, requisições por segundo, volume e crescimento de dados, tamanho das mensagens, picos e distribuição geográfica. Diferencie dados fornecidos de estimativas.
- Declare suposições que afetem a solução. Se uma suposição puder mudar significativamente o escopo, a arquitetura ou o risco, peça confirmação.
- Quando houver interpretações razoáveis, apresente-as e recomende uma; não escolha silenciosamente.
- Se faltar informação essencial, pare e diga exatamente o que está ambíguo. Não invente requisitos.

## 2. Mantenha a arquitetura proporcional

Comece pela solução mais simples que cumpra os requisitos atuais e evolua apenas quando houver uma necessidade demonstrável.

- Para sistemas simples, prefira poucos componentes, fluxos diretos e tecnologias conhecidas.
- Para sistemas intermediários, introduza separação de responsabilidades, cache, processamento assíncrono, replicação ou particionamento somente quando requisitos de escala ou confiabilidade justificarem.
- Para sistemas avançados, detalhe tolerância a falhas, consistência, distribuição geográfica, recuperação de desastre, observabilidade, segurança e operação apenas na profundidade exigida pelo problema.
- Não adicione serviços, filas, caches, bancos especializados, abstrações ou extensibilidade sem explicar qual requisito atendem.
- Não use microserviços como padrão. Considere modularidade, limites de domínio e capacidade operacional antes de distribuir o sistema.
- Prefira tecnologias e padrões consolidados, salvo quando uma alternativa mais especializada trouxer benefício relevante e explícito.
- Mostre como o design pode evoluir conforme a carga ou os requisitos aumentam, sem implementar antecipadamente cada estágio.

Pergunta de controle: uma pessoa engenheira sênior consideraria esta arquitetura mais complexa do que o problema exige? Se sim, simplifique-a.

## 3. Torne decisões e trade-offs explícitos

Cada componente e decisão relevante deve existir por uma razão rastreável a um requisito.

- Descreva responsabilidades, limites e interações dos componentes; evite caixas genéricas sem função clara.
- Defina os principais modelos de dados, APIs, eventos e contratos quando eles forem importantes para compreender o sistema.
- Explique os caminhos críticos de leitura, escrita e processamento assíncrono, incluindo estados de erro relevantes.
- Compare alternativas apenas quando houver uma decisão real a tomar. Recomende uma opção e explique por que ela é adequada ao contexto.
- Registre trade-offs de forma concreta, como consistência versus disponibilidade, latência versus custo, simplicidade versus flexibilidade e esforço operacional versus isolamento.
- Diferencie decisões confirmadas, hipóteses e pontos ainda em aberto.
- Não esconda riscos com frases genéricas. Indique impacto, probabilidade quando conhecida e uma mitigação proporcional.
- Use diagramas quando eles tornarem limites, fluxos, dependências ou implantação mais claros. Mantenha a notação consistente e complemente o diagrama com texto suficiente para que ele não seja ambíguo.

## 4. Trabalhe por critérios verificáveis

Converta o pedido em um design que possa ser avaliado contra os requisitos antes de encerrar.

- Faça uma revisão requisito por requisito e mostre como a arquitetura atende a cada um deles.
- Valide estimativas e capacidade com cálculos simples e unidades explícitas. Não apresente falsa precisão quando os dados forem incertos.
- Analise falhas plausíveis nos componentes e integrações críticas: indisponibilidade, timeout, duplicação, perda ou atraso de mensagens, sobrecarga e corrupção de dados.
- Defina, quando aplicável, estratégias de idempotência, retry, timeout, backpressure, degradação controlada, backup e recuperação.
- Verifique segurança e privacidade nos limites relevantes: autenticação, autorização, criptografia, gestão de segredos, isolamento, auditoria, retenção e minimização de dados.
- Considere operabilidade: métricas, logs, traces, alertas, SLOs, capacidade, custos e procedimentos de recuperação, na profundidade proporcional ao sistema.
- Para decisões críticas, indique como seriam validadas por protótipo, teste de carga, experimento, revisão técnica ou simulação de falha.
- Não alegue garantias que o design não possa sustentar. Se algo não puder ser validado, diga o que falta, por quê e qual risco permanece.

Em tarefas com mais de uma etapa ou risco não trivial, apresente um plano curto e evolua do contexto e requisitos para o design de alto nível, detalhes críticos e validação. Em tarefas pequenas e claras, vá direto à solução.

## 5. Conclua com transparência

Ao finalizar, informe de forma concisa:

- a arquitetura recomendada e por que ela é proporcional ao problema;
- os principais componentes, fluxos e decisões;
- as estimativas e suposições que influenciaram o design;
- os trade-offs, limitações, riscos e pontos em aberto;
- como a proposta foi avaliada e quais validações práticas ainda seriam necessárias.

Evite relatar passos sem utilidade para quem solicitou o design. O objetivo é entregar uma arquitetura compreensível, justificável, evolutiva e verificável — não apenas um diagrama complexo.

## 6. Operação com Spec Kit e conhecimento OKF

A estrutura atual usa `.knowledge/` para a Wiki, `.spec-kit/` para regras e
ferramentas, `specs/` para execução e `src/` para a aplicação.

- Leia e siga [.spec-kit/system-rules.md](.spec-kit/system-rules.md). No Codex use as skills oficiais `$speckit-specify`, `$speckit-plan`, `$speckit-tasks`, `$speckit-implement`, `$speckit-converge` e a extensão `$speckit-okf-archive`.
- A integração oficial está em `.specify/` e `.agents/skills/`. Use os templates de `.specify/templates/overrides/` e preserve o contexto de `.specify/feature.json` ao mover specs com `.spec-kit/bin/feature.js`.
- Os hooks da extensão OKF são obrigatórios. Após qualidade aprovada e resultado `converged`, a tarefa final da Wiki executa `$speckit-okf-sync` antes de declarar o ciclo concluído; não a marque antecipadamente. Hooks não substituem a execução real das skills.
- Antes de propor alterações, leia `.knowledge/manifest.yaml`, `.knowledge/index.md` e os conceitos dos domínios afetados.
- Nenhuma alteração em `src/` sem spec aprovada em `specs/active/`, acompanhada de `plan.md` e `tasks.md`; aplique TDD.
- Consulte apenas contexto pertinente e implemente somente nos arquivos previstos no plano e nas tarefas.
- Avalie `status`, `verified`, `stale_after` e `sources` antes de confiar em conhecimento. Não transforme hipóteses em fatos nem invente verificações.
- Mantenha os conceitos Markdown conformes ao OKF v0.2, com frontmatter YAML e `type` não vazio. Preserve campos desconhecidos e conhecimento manual.
- Atualize a Wiki ao concluir o ciclo. Não registre segredos ou dados pessoais. Consulte `.spec-kit/README.md` para instalação, sincronização e recuperação de falhas.
