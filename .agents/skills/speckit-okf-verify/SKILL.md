---
name: speckit-okf-verify
description: Verificar o lote implementado sem promover intenção a conhecimento atual.
compatibility: Requires spec-kit project structure with .specify/ directory
metadata:
  author: github-spec-kit
  source: okf:commands/verify.md
---

Leia `.spec-kit/system-rules.md` e o escopo solicitado a implement. Execute os
comandos de qualidade previstos para esse lote e registre resultados reais e
evidências de vermelho/verde nos artefatos. Se uma verificação falhar, interrompa
o avanço e reporte a falha; não marque testes como aprovados.

Respeite o limite de fase/IDs fornecido pelo desenvolvedor. Não execute tarefas
de outras fases nem a tarefa final da Wiki. Ela é uma tarefa do encerramento do
ciclo, adiada até converge; sua pendência não impede encerrar um lote.

Com tarefas de código restantes, mantenha status approved e informe o próximo
lote. Com todo o trabalho executável concluído e qualidade final aprovada, use
status verified e tests_passed true, com quality.tests, coverage, lint, types,
build e acceptance. Para types/build não aplicáveis, registre applicable false
e reason concreta definida no plano; não invente um comando aprovado.

Um commit local pode registrar o lote somente nos arquivos previstos,
preservando staged alheio, sem push. Recomende $speckit-converge após
a validação final. Não execute sync nem declare completed nesta etapa.