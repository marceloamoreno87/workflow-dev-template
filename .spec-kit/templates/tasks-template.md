# Tarefas: {{FEATURE}}

Cada tarefa deve nomear arquivos do plano e requisitos/cenários cobertos.
Use IDs T001, T002 etc. e [US1] para vincular histórias. Agrupe por história,
testes antes do código. [P] indica independência, não autorização para subagentes.
Testes são obrigatórios neste projeto, conforme a constituição.

## User Story 1 (P1)

- [ ] T001 [US1] Criar testes de aceitação e regressão em [arquivo]; confirmar falha esperada.
- [ ] T002 [US1] Implementar módulo em [arquivo] para satisfazer os testes.
- [ ] T003 [US1] Integrar contratos e cenários de erro em [arquivo].

## Validação e conhecimento

- [ ] T004 Refatorar e executar testes, cobertura, lint e tipos do plan.md.
- [ ] T005 Revisar a diff e registrar evidências/testes/status em spec.md e tasks.md.
- [ ] T006 Atualizar a LLM Wiki em .knowledge/ com as mudanças realizadas neste ciclo

Execute T006 com $speckit-okf-sync; deixe pendente até esse passo concluir.
Se $speckit-converge acrescentar tarefas, execute-as e refaça o sync antes de arquivar.

## Evidências

- Vermelho: [comando, falha e data]
- Verde: [comando, resultado e data]
- Cobertura, lint e tipos: [comandos e resultados]
- Wiki: [SHA do commit automático; preencher após sincronizar]
