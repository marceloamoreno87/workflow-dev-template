# Tarefas: {{FEATURE}}

Cada tarefa deve nomear arquivos do plano e requisitos/cenários cobertos.
Use IDs T001, T002 etc. e [US1] para vincular histórias. Agrupe por história,
testes antes do código. [P] indica independência, não autorização para subagentes.
Testes são obrigatórios neste projeto, conforme a constituição.

## User Story 1 (P1)

- [ ] T001 [US1] Criar testes de aceitação e regressão em [arquivo]; confirmar falha esperada.
- [ ] T002 [US1] Implementar módulo em [arquivo] para satisfazer os testes.
- [ ] T003 [US1] Integrar contratos e cenários de erro em [arquivo].

## Validação executável

- [ ] T004 Refatorar e executar testes, cobertura, lint, tipos, build e aceitação do plan.md.
- [ ] T005 Revisar a diff e registrar evidências/testes/status em spec.md e tasks.md.
## Encerramento pós-convergência

- [ ] T006 Atualizar a LLM Wiki em .knowledge/ com as mudanças realizadas neste ciclo

T006 é tarefa do encerramento do ciclo: implement deve deixá-la pendente.
Execute $speckit-converge após validar o código. Se acrescentar tarefas, repita
implement → verify → converge. Somente resultado converged autoriza T006 via
$speckit-okf-sync, executada pelo hook after_converge. Não duplique a tarefa Wiki.

Para execução parcial, delimite fases/IDs e comandos por lote. Não avance além
do intervalo solicitado; falha de verificação bloqueia a fase seguinte.

## Evidências

- Vermelho: [comando, falha e data]
- Verde: [comando, resultado e data]
- Cobertura, lint e tipos: [comandos e resultados]
- Wiki: [SHA do commit automático; preencher após sincronizar]
