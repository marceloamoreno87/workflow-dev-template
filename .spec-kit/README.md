# Spec Kit oficial + Wiki OKF v0.2

Este é o guia técnico de manutenção. Para uso diário e visão completa, consulte
o [guia rápido](../QUICKSTART.md) e o [README principal](../README.md).

Integração Codex do Specify CLI 1.0.6, com extensão local OKF. Pronta neste
repositório: skills em `.agents/skills/`, core em `.specify/`, Wiki em `.knowledge/`.

## Uso no Codex

Abra uma nova sessão na raiz do projeto. Digite `$` para ver as skills.

1. `$speckit-specify Quero criar ...` — descreva a necessidade; gera backlog.
2. Revise a spec e diga: `Aprovado. $speckit-plan` — gera plano com contexto OKF.
3. `$speckit-tasks` — gera tarefas TDD e ativa a spec.
4. `$speckit-implement` — implementa, testa e chama a revisão/sync da Wiki.
5. `$speckit-converge` — verifica aderência; repita implementação se houver tarefas.
6. `$speckit-okf-archive` — valida conclusão/Wiki e arquiva.

## Projeto novo e projeto existente

O ciclo acima atende ambos. Em projeto novo, a primeira spec e seu plano definem
a stack mínima, a estrutura e os primeiros domínios a partir dos requisitos. Em
projeto existente, faça antes uma descoberta dirigida ao domínio afetado:
confirme stack, caminhos, contratos, integrações e comandos de qualidade; se não
houver proteção suficiente, inclua testes de caracterização nas tarefas.

Não é necessário mapear todo o legado antes da primeira feature. Registre na
Wiki somente conhecimento sustentado por código, testes, documentação confiável
ou outra fonte explícita, avaliando `status`, `verified`, `stale_after` e
`sources`. O sync cobre a diff desde o `base_commit`; ele não reconstrói
automaticamente toda a arquitetura anterior.

A constituição já está preenchida. `$speckit-clarify` e `$speckit-analyze` são
opcionais. `$speckit-okf-sync` retoma uma sincronização pendente. Não use os
antigos `/specify`, `/plan`, `/tasks`: não são os comandos desta integração.

## Instalação em outro clone

Requisitos: Specify CLI 1.0.6, Bash, Git e Node.js >= 18.

```sh
npm ci --prefix .spec-kit
specify integration list
specify extension list
.spec-kit/bin/install-hooks.sh
node .spec-kit/bin/validate-okf.js
npm test --prefix .spec-kit
npm run audit --prefix .spec-kit
```

As skills geradas são versionáveis. Ao editar a extensão em
`.spec-kit/extensions/okf/`, registre novamente com:

```sh
specify extension add --dev .spec-kit/extensions/okf --force
```

Os templates canônicos são `.specify/templates/overrides/`. O core oficial
permanece em `.specify/templates/`; não edite as skills oficiais geradas.
Após upgrade do Spec Kit, confirme overrides, hooks, extensão e testes.

## Atalhos de terminal

```sh
.spec-kit/bin/new-spec.sh minha-feature
.spec-kit/bin/fast-track.sh corrigir-login
node .spec-kit/bin/feature.js select specs/backlog/minha-feature
```

Os atalhos criam rascunhos e selecionam a feature; aprovação continua obrigatória.
Fast-track tem documentos curtos. O hook before_implement valida e registra sua
base antes de permitir código. `feature.js` preserva `.specify/feature.json`
ao mover pastas e remove somente o link de fast-track ao arquivar.

## Sincronização da Wiki

O Codex analisa a diff inteira da feature, edita os conceitos e registra um
recibo. O script valida YAML/recibo, atualiza proveniência/índice/versão e comita
somente a Wiki. Não depende de chave de API ou de outra sessão de IA.

```sh
node .spec-kit/bin/auto-sync-wiki.js --prepare
# Após revisar código, editar conceitos e registrar testes concluídos:
node .spec-kit/bin/auto-sync-wiki.js --reviewed-by codex/session
node .spec-kit/bin/auto-sync-wiki.js
```

O primeiro comando fornece contexto; os seguintes são usados pelo agente.
`--reviewed-by` declara revisão efetivamente realizada, não a executa sozinho.
`wiki-sync.json` vincula commit, base, plano e hashes dos documentos. Mudanças
posteriores exigem nova revisão. Nunca use o recibo como prova de correção semântica.
Cada domínio afetado deve ser editado. O sync exige evidências estruturadas de
testes, cobertura, lint e tipos, e remove `verified` que ficou obsoleto.

O post-commit executa em segundo plano apenas a publicação de revisão já pronta.
Sem recibo correspondente, registra pendência; o hook after_implement do Spec
Kit faz a revisão na sessão atual. Log: `git rev-parse --git-path okf-auto-sync.log`.
Trava: `git rev-parse --git-path okf-auto-sync.lock`; confira o PID antes de remover
trava residual. Em falha de commit, corrija e repita a publicação. Não há push.

O archive valida todo o estado e cria um commit local apenas dos artefatos da
spec. Use `npm run audit --prefix .spec-kit` para conferir a instalação completa.

## OKF

`index.md` declara a versão 0.2. Conceitos têm frontmatter YAML com `type`.
O manifesto é uma extensão local para domínios, stack e qualidade; suporta YAML
normal. `knowledge_version` é independente de `okf_version` e `system.version`.

Referências: [Spec Kit](https://github.com/github/spec-kit),
[integração Codex](https://github.github.io/spec-kit/reference/integrations.html),
[OKF v0.2](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).
