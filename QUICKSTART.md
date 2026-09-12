# Guia rápido

Abra uma nova sessão do Codex na raiz do projeto e digite `$` para localizar as
skills. Trabalhe em uma feature por vez e não altere `src/` antes da ativação.

## Fluxo recomendado

1. Especifique:

   ```text
   $speckit-specify Quero permitir redefinição de senha por link temporário.
   ```

2. Revise `spec.md`. Se necessário, execute `$speckit-clarify`. Depois aprove:

   ```text
   Aprovado. Registre minha aprovação e execute $speckit-plan.
   ```

3. Gere e revise as tarefas:

   ```text
   $speckit-tasks
   $speckit-analyze
   ```

4. Implemente com testes primeiro:

   ```text
   $speckit-implement
   ```

5. Confirme aderência e arquive:

   ```text
   $speckit-converge
   $speckit-okf-archive
   ```

Se converge criar tarefas, repita implement e converge antes de arquivar.

## Boas práticas essenciais

- Especifique comportamento e valor; deixe tecnologia para o plano.
- Aprove a spec explicitamente antes do planejamento.
- Confirme a falha dos testes antes da implementação e registre evidências.
- Altere somente arquivos previstos em `plan.md` e `tasks.md`.
- Não marque aprovação, testes ou tarefas sem evidência real.
- Mantenha apenas uma feature ativa e comece com `src/`/`.knowledge/` limpos.
- Registre comandos de testes, cobertura, lint e tipos no bloco `quality` da spec.
- Deixe a tarefa da Wiki para `$speckit-okf-sync`; o fluxo normal a executa ao final.
- Execute `$speckit-converge` antes de arquivar.
- Não edite `wiki-sync.json` nem faça push automático.

Para uma correção pequena:

```sh
.spec-kit/bin/fast-track.sh corrigir-timeout
```

Preencha e aprove os documentos, execute `$speckit-okf-activate` e prossiga com
`$speckit-implement`. Fast-track também exige TDD e atualização da Wiki.

Diagnóstico rápido:

```sh
npm test --prefix .spec-kit
npm run validate --prefix .spec-kit
npm run audit --prefix .spec-kit
specify integration list
specify extension list
```

Veja pastas, todas as skills, etapas e recuperação no [README completo](README.md).
