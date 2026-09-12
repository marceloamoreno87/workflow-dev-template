---
type: Architecture Decision
title: Integração Spec Kit e Wiki OKF
status: stable
generated:
  by: codex/session
  at: 2026-09-12T00:00:00Z
sources:
  - resource: https://github.github.io/spec-kit/reference/integrations.html
  - resource: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
---
# Decisão

Usar o Specify CLI 1.0.6 com integração Codex em modo skills. Preservar o core
oficial em `.specify/` e personalizar templates por overrides. A extensão local
OKF integra consulta de conhecimento, ciclo de specs e revisão da Wiki por hooks.

O contexto de feature fica em `.specify/feature.json`; mover uma spec atualiza
esse ponteiro. Backlog contém rascunhos, active contém specs aprovadas e archive
contém ciclos concluídos. Ativação registra a base Git para revisão de todos os
commits da feature.

O Codex faz a síntese semântica durante a implementação. O sincronizador local
valida YAML, atualiza proveniência/índice, confere um recibo de hashes e comita
somente a Wiki. O hook Git apenas publica revisões prontas e não chama uma LLM.
Isso usa a sessão atual, sem chave de API adicional.

# Limites

O recibo detecta alteração dos artefatos após a revisão, mas não comprova
correção semântica nem substitui testes. O manifesto é uma extensão local,
enquanto os conceitos Markdown e o índice seguem OKF v0.2. A stack da aplicação
permanece em aberto até a primeira spec.
