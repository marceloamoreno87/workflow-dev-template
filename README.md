# Workflow Spec Kit + Codex + Wiki OKF v0.2

Este repositório usa o GitHub Spec Kit 1.0.6 para desenvolvimento orientado por
especificações. O Codex executa o fluxo por skills locais, enquanto a extensão
OKF consulta e atualiza a Wiki de conhecimento junto com cada feature.

Para começar sem ler todo este guia, consulte o [guia rápido](QUICKSTART.md).
Para um exemplo completo, consulte o [fluxo avançado](ADVANCED-WORKFLOW.md).

## Estado inicial do template

O template não contém specs, conhecimento compilado, recibos de execução ou
constituição ratificada. Skills, scripts, templates, registros de instalação e
configuração da extensão são componentes do workflow e permanecem disponíveis.
Exemplos nos guias são ilustrativos, não features executadas.

Ao criar um projeto, preencha `system.name` e `system.version` em
`.knowledge/manifest.yaml` e gere sua constituição com `$speckit-constitution`.
Revise a política de qualidade fornecida como padrão. Mantenha `domains: []`,
`knowledge_version: "0.0.0"` e `last_synced_commit: null` até haver conhecimento
real para registrar. A Wiki começa apenas com índice e manifesto.

Antes de distribuir este template, execute
`npm run audit:template --prefix .spec-kit`. Essa verificação exige estado vazio;
projetos consumidores usam `npm run audit --prefix .spec-kit` para verificar a
instalação do workflow.

## Visão geral

| Área | Responsabilidade |
| --- | --- |
| `.specify/` | Núcleo oficial, integração Codex, constituição, templates e feature atual. |
| `.agents/skills/` | Skills que aparecem no Codex e executam o workflow. |
| `.knowledge/` | Wiki OKF v0.2: arquitetura, domínios e interfaces. |
| `specs/` | Especificação, plano, tarefas, evidências e recibo de cada ciclo. |

```text
necessidade → spec → aprovação → plano → tarefas → implementação TDD
            → verificação → convergência → revisão da Wiki → arquivo
```

## Adoção por tipo de projeto

O workflow atende projetos novos e projetos existentes. Depois da preparação
inicial, ambos seguem o mesmo ciclo por feature. O que muda é a forma de criar
uma baseline confiável antes da primeira implementação.

| Aspecto | Projeto novo | Projeto existente |
| --- | --- | --- |
| Wiki inicial | Nasce com os primeiros domínios | Começa com o contexto do legado efetivamente verificado |
| Stack | Definida no primeiro plano | Registrada a partir do repositório e da operação reais |
| Arquitetura | Evolui a partir da solução mínima | É descoberta antes de ser modificada |
| Testes | TDD desde o primeiro comportamento | Caracterização do legado seguida de TDD para a mudança |
| Maior risco | Projetar antecipadamente | Tratar documentação ou hipótese como comportamento real |

### Projeto novo, do zero

1. Gere a constituição do novo projeto com `$speckit-constitution`.
2. Descreva objetivos, atores, fluxo principal, restrições e escala conhecida na
   primeira `$speckit-specify`.
3. Use `$speckit-clarify` para decisões que alterem arquitetura ou escopo.
4. Após aprovação, use `$speckit-plan` para definir a menor stack e estrutura que
   atendam aos requisitos, bem como comandos reais de testes, cobertura, lint, tipos, build e aceitação.
5. Registre os primeiros domínios, contratos e decisões na Wiki por meio do ciclo
   normal de implementação e sync.

Não preencha antecipadamente toda a arquitetura. A Wiki e o manifesto devem
crescer conforme decisões comprovadas por features reais.

### Projeto existente, em andamento

Antes da primeira feature, faça uma descoberta dirigida ao domínio que será
alterado. Não é necessário documentar todo o legado de uma vez.

1. Confirme a stack, a estrutura do repositório e os comandos reais de qualidade.
2. Identifique os módulos, contratos, integrações, dados e procedimentos
   operacionais afetados pela próxima mudança.
3. Classifique cada informação como confirmada por teste, observada no código,
   proveniente de documentação ou ainda hipotética.
4. Registre no manifesto e nos conceitos OKF somente conhecimento sustentado por
   fontes. Use `verified` apenas quando houver verificação comprovável e avalie
   `status`, `stale_after` e `sources`.
5. Quando faltar proteção automatizada, inclua testes de caracterização no plano
   e nas tarefas antes de mudar o comportamento existente.
6. Crie a primeira spec da mudança; daí em diante, siga o ciclo comum.

O `$speckit-okf-sync` revisa a diff da feature a partir do `base_commit`. Ele não
reconstrói automaticamente todo o histórico ou toda a arquitetura preexistente.
Conhecimento fora do domínio afetado deve ser incorporado em ciclos posteriores,
quando for consultado e verificado.

### Ciclo comum após a adoção

```text
specify → clarify → aprovação → plan → tasks → analyze
        → implement → verify → converge → sync → archive
```

Em ambos os cenários, nenhuma alteração em `src/` ocorre antes de uma spec
aprovada, plano, tarefas e ativação. A primeira ativação também exige `src/` e
`.knowledge/` limpos para registrar uma base não contaminada.

## Pré-requisitos e verificação

- Git, Bash, npm e Node.js 18 ou superior;
- Specify CLI 1.0.6;
- Codex CLI ou extensão do Codex com skills locais.

```sh
specify version
specify integration list
specify extension list
npm ci --prefix .spec-kit
npm test --prefix .spec-kit
npm run validate --prefix .spec-kit
npm run audit --prefix .spec-kit
```

O resultado deve mostrar `codex` como integração padrão, a extensão `okf`
habilitada, testes verdes e o bundle OKF válido.

## Estrutura

```text
.
├── .agents/                 # Skills descobertas pelo Codex
│   └── skills/
├── .knowledge/              # Wiki OKF v0.2
│   ├── manifest.yaml
│   ├── index.md
│   ├── architecture/
│   ├── domains/
│   └── interfaces/
├── .specify/                # Spec Kit oficial e estado operacional
│   ├── extensions.yml
│   ├── feature.json         # Criado ao selecionar uma feature; ignorado pelo Git
│   ├── integrations/
│   ├── memory/
│   ├── scripts/
│   ├── templates/overrides/
│   └── workflows/
├── .spec-kit/               # Extensão OKF e automação local
│   ├── bin/
│   ├── extensions/okf/
│   ├── hooks/
│   ├── templates/
│   ├── tests/
│   └── system-rules.md
├── specs/
│   ├── backlog/
│   ├── active/
│   └── archive/
├── src/                     # Código da aplicação
├── AGENTS.md                # Regras carregadas pelo Codex
├── QUICKSTART.md            # Uso diário resumido
└── README.md                # Este guia
```

### `.agents/skills/`

Contém as skills oficiais geradas pelo Specify CLI e as skills da extensão OKF.
Digite `$` no chat para abrir o seletor. Se uma skill nova não aparecer, abra
uma nova sessão do Codex na raiz do projeto.

Esses arquivos são gerados. Personalize templates, constituição ou a extensão
em `.spec-kit/extensions/okf/` e registre-a novamente. Uma atualização pode
substituir edições feitas diretamente nas skills.

### `.specify/`

É o diretório que identifica este repositório como projeto Spec Kit.

- `integration.json`: registra o Codex como integração padrão.
- `extensions.yml`: habilita os hooks obrigatórios da Wiki.
- `feature.json`: aponta para a feature atual e não deve ser movido manualmente.
- `memory/constitution.md`: princípios obrigatórios do projeto.
- `scripts/bash/`: scripts oficiais usados pelas skills.
- `templates/`: templates oficiais da versão instalada.
- `templates/overrides/`: templates canônicos deste projeto, com aprovação,
  TDD, impacto OKF e tarefa final da Wiki.
- `workflows/`: definição do workflow instalado.
- `extensions/okf/`: cópia instalada da extensão local.

### `.knowledge/`

É a Wiki consultada antes das decisões e atualizada depois do código.

- `index.md`: índice raiz, com `okf_version: "0.2"`.
- `manifest.yaml`: sistema, stack, versão do conhecimento, domínios,
  dependências, caminhos de código e requisitos de qualidade.
- `architecture/`: decisões arquiteturais e padrões duráveis.
- `domains/`: responsabilidades, regras e invariantes dos domínios.
- `interfaces/`: APIs, eventos, modelos e contratos compartilhados.

Todo conceito Markdown, exceto `index.md` e `log.md`, deve ter frontmatter YAML
com `type` não vazio. Use `status`, `generated`, `sources`, `verified` e
`stale_after` quando aplicáveis. `verified` exige verificação real; automação não
deve inventar revisão humana.

O manifesto é uma extensão local de descoberta. A estrutura dos conceitos e o
índice seguem o OKF v0.2.

### `.spec-kit/`

Contém o código mantido pelo projeto para ligar o Spec Kit à Wiki.

- `system-rules.md`: regras de execução do agente.
- `extensions/okf/`: fonte da extensão local e de seus hooks.
- `templates/`: fontes dos overrides, versões fast-track e roteiro de sync.
- `bin/feature.js`: seleciona e move features mantendo `feature.json`.
- `bin/convergence.js`: registra convergência após a avaliação real do agente.
- `bin/auto-sync-wiki.js`: prepara a diff, registra revisão e publica a Wiki.
- `bin/validate-okf.js`: valida manifesto e conceitos OKF.
- `bin/audit-workflow.js`: confere integração, skills, hooks, templates e estado.
- `bin/new-spec.sh`: cria manualmente uma spec no backlog.
- `bin/fast-track.sh`: cria um ciclo curto para correções pequenas.
- `bin/install-hooks.sh`: instala o hook Git sem substituir outro hook.
- `hooks/post-commit`: tenta publicar uma revisão já preparada.
- `tests/`: valida integração, ciclo, sincronização e recuperação.

### `specs/`

Cada pasta representa um ciclo e normalmente contém:

```text
spec.md                 # O que será construído e por quê
plan.md                 # Como construir e quais domínios são afetados
tasks.md                # Trabalho executável, com testes primeiro
checklists/             # Qualidade dos requisitos
research.md             # Decisões técnicas opcionais
data-model.md           # Entidades e relacionamentos opcionais
contracts/              # Contratos específicos da feature
quickstart.md           # Cenários de validação opcionais
convergence-report.md   # Avaliação de aderência feita pelo agente
convergence.json        # Estado vinculado à avaliação de convergência
wiki-sync.json          # Recibo da revisão final da Wiki
```

- `backlog/`: rascunho; não autoriza alteração em `src/`.
- `active/`: spec aprovada, com plano e tarefas, pronta para implementação.
- `archive/`: ciclo convergido e com Wiki sincronizada.

Specs arquivadas são histórico. Abra outro ciclo para uma nova mudança.

### `src/` e `AGENTS.md`

`src/` contém o código da aplicação. Alterações exigem uma spec aprovada em
`active`, plano, tarefas e TDD. `AGENTS.md` é carregado pelo Codex ao iniciar e
aponta para essas regras. Reinicie a sessão após alterar instruções ou skills.

## Skills

### Fluxo principal

| Skill | Quando usar | Resultado |
| --- | --- | --- |
| `$speckit-constitution` | Ao iniciar ou alterar princípios. | Preenche o template com os princípios do projeto. |
| `$speckit-specify <descrição>` | Ao iniciar uma feature. | Cria uma spec testável; o hook a move para backlog. |
| `$speckit-clarify` | Quando requisitos relevantes estiverem ambíguos. | Faz perguntas focadas e atualiza a spec. |
| `$speckit-plan` | Após aprovação explícita. | Gera arquitetura, contratos, pesquisa, modelo e impacto OKF. |
| `$speckit-tasks` | Quando o plano estiver pronto. | Gera tarefas TDD; o hook ativa a feature e registra a base. |
| `$speckit-analyze` | Antes da implementação. | Analisa lacunas entre spec, plano e tarefas sem mudar código. |
| `$speckit-implement` | Após análise e ativação. | Executa o lote com TDD; verifica qualidade e mantém Wiki pendente. |
| `$speckit-converge` | Depois da implementação. | Acrescenta gaps; só converged registra revisão e chama sync. |
| `$speckit-okf-archive` | Após convergência, testes e sync. | Valida e move a feature para archive. |

### Skills auxiliares

| Skill | Finalidade |
| --- | --- |
| `$speckit-checklist` | Cria checklist específica de qualidade dos requisitos. |
| `$speckit-taskstoissues` | Converte tarefas em issues; exige intenção de escrever no GitHub. |
| `$speckit-okf-context` | Hook que consulta manifesto, índice, domínios, contratos e ADRs. |
| `$speckit-okf-stage` | Hook que move uma nova spec para backlog e atualiza o contexto. |
| `$speckit-okf-activate` | Valida aprovação/artefatos, ativa e registra o commit base. |
| `$speckit-okf-verify` | Verifica o lote, respeita IDs/fases e mantém Wiki pendente. |
| `$speckit-okf-converged` | Registra convergência real e só então chama sync. |
| `$speckit-okf-sync` | Revisa a diff completa e sincroniza a Wiki; retoma falhas. |

As skills OKF internas são chamadas pelos hooks. Também podem ser usadas
manualmente para diagnóstico e recuperação.

## Etapas do workflow

### 1. Especificar

```text
$speckit-specify Permitir redefinição de senha por link de uso único e prazo limitado.
```

A skill consulta a Wiki, cria uma feature e gera requisitos, cenários, resultados
mensuráveis e checklist. O hook stage move o ciclo para backlog e atualiza
`feature.json`. Descreva comportamento e valor; deixe tecnologia para o plano.

### 2. Esclarecer e aprovar

Leia `spec.md`. Para dúvidas importantes, execute `$speckit-clarify`. Quando o
documento estiver correto, aprove explicitamente:

```text
Aprovado. Registre minha aprovação nesta spec e execute $speckit-plan.
```

Isso preenche `status: approved` e `approved_by`. Criar ou mover a pasta não
equivale a aprovação.

### 3. Planejar

Execute `$speckit-plan`. Preencha Knowledge Context com conceitos, fontes,
validade, restrições, lacunas e vínculo com decisões/requisitos. O plano define stack, arquitetura, arquivos permitidos,
comandos de qualidade e estratégia de teste. Tipos/build não aplicáveis exigem
`applicable: false` e justificativa concreta; aceitação continua obrigatória. Também mapeia domínios:

```json
[
  {
    "id": "identidade",
    "path": "domains/identidade.md",
    "depends_on": ["notificacoes"],
    "source_paths": ["src/identity/"],
    "summary": "Responsabilidades previstas para recuperação de acesso."
  }
]
```

`path` é relativo a `.knowledge/`. `source_paths` aceita arquivo exato ou
diretório terminado em `/`, sem glob. Todo arquivo alterado em `src/` deve
pertencer a um domínio.

### 4. Gerar tarefas e ativar

Execute `$speckit-tasks`. As tarefas recebem IDs `T001`, vínculos `[US1]`,
caminhos concretos e dependências. Testes vêm antes do código e a Wiki é a tarefa
final. O hook activate move a feature para `active` e registra `base_commit`,
permitindo revisar todos os commits do ciclo. Só uma feature pode ficar ativa;
`src/` e `.knowledge/` precisam estar limpos na primeira ativação.

### 5. Analisar

Execute `$speckit-analyze`. Corrija requisitos sem tarefa, tarefas sem requisito,
divergências de caminho e contradições. A análise não altera código.

### 6. Implementar com TDD

Execute `$speckit-implement`. Para cada comportamento:

1. escreva o teste conforme a spec;
2. execute e confirme a falha esperada;
3. implemente o mínimo necessário;
4. execute novamente;
5. refatore mantendo testes verdes;
6. rode cobertura, lint, tipos, build e aceitação definidos no plano;
7. registre em `spec.md` os comandos/resultados de testes, cobertura, lint, tipos, build e aceitação; a cobertura deve atingir o mínimo do manifesto;
8. marque apenas tarefas concluídas.

Se surgir arquivo ou domínio fora do plano, atualize plano e tarefas primeiro.
Mudanças de escopo exigem decisão do desenvolvedor.

### 7. Verificar convergência

Execute `$speckit-converge` após a qualidade final. Ele avalia a realidade contra
spec, plano, tarefas e constituição. Se acrescentar tarefas, repita implement →
verify → converge. A única tarefa da Wiki é encerramento diferido, não gap de
código. O core converge mantém os artefatos de intenção; o hook local registra
convergence-report.md e convergence.json somente após resultado converged.

Esse recibo vincula código/testes, spec/plano/tarefas e relatório. Alterações
posteriores exigem nova avaliação. Ele registra a revisão realizada, sem provar
correção semântica automaticamente.

### 8. Sincronizar a Wiki

Somente resultado converged autoriza o hook `after_converge` a chamar
`$speckit-okf-sync` e concluir a tarefa final. O Codex lê
a diff entre `base_commit` e o commit final, confronta código e conceitos,
atualiza conhecimento durável e grava `wiki-sync.json`. O script valida YAML,
proveniência e hashes, cria um commit só da Wiki e então marca a tarefa.
Cada domínio afetado precisa ser semanticamente editado. Uma alteração remove
`verified` antigo, pois aquela verificação não cobre o conteúdo novo.

O hook `post-commit` apenas tenta publicar uma revisão já preparada; não inicia
uma IA. Sem recibo, registra a pendência no log. Para retomar, execute
`$speckit-okf-sync`.

Diagnóstico:

```sh
node .spec-kit/bin/auto-sync-wiki.js --prepare
git rev-parse --git-path okf-auto-sync.log
git rev-parse --git-path okf-auto-sync.lock
node .spec-kit/bin/validate-okf.js
```

### 9. Arquivar

Com convergência e sync válidos, execute `$speckit-okf-archive`. Se o recibo de
convergência ficou obsoleto, refaça converge e resolva novas tarefas antes de
seguir. O arquivo valida testes, código local, recibo,
manifesto e hashes antes de mover a pasta para archive, então cria um commit
local exclusivo dos artefatos da spec. Nenhuma automação faz push.

## Fast-track

```sh
.spec-kit/bin/fast-track.sh corrigir-timeout
```

Preencha e aprove os três documentos curtos, execute `$speckit-okf-activate` e
prossiga com `$speckit-implement`. Fast-track continua exigindo TDD e Wiki. Use-o
apenas para problemas pequenos e bem compreendidos; mudanças arquiteturais ou
com escopo incerto seguem o fluxo completo.

## Boas práticas

- Mantenha uma feature ativa por contexto. Para trocar, use
  `node .spec-kit/bin/feature.js select specs/<estado>/<nome>`.
- Você pode enfileirar specs no backlog; enquanto houver uma feature ativa, o
  ponteiro operacional permanece nela.
- Escreva specs com comportamento observável e valor para o usuário.
- Aprove a spec antes de decidir tecnologia.
- Use clarify para escolhas que mudam escopo, segurança ou experiência.
- Execute analyze antes do código e converge antes de arquivar.
- Crie tarefas pequenas, ligadas a histórias e com caminhos concretos.
- Registre a fase vermelha; teste escrito depois não comprova TDD.
- Não marque aprovação, testes ou tarefas sem evidência.
- Não inicie outra feature enquanto existir uma pasta em `specs/active/`.
- A ativação exige Wiki limpa; mudanças futuras só entram no OKF após converge.
- Evite misturar duas features no intervalo iniciado por `base_commit`.
- Não use `git add .` no sync; preserve alterações staged que não pertencem à Wiki.
- Não edite `wiki-sync.json` manualmente.
- Atualize conhecimento durável, sem progresso transitório, segredos ou dados pessoais.
- Preserve campos desconhecidos, fontes anteriores e conceitos históricos.
- Use `deprecated` para conceitos aposentados que ainda recebem links.
- Não altere `okf_version` no sync; ele incrementa `knowledge_version`.
- Revise commits localmente e faça push conscientemente; o hook não faz push.
- Após atualizar o Spec Kit, valide overrides, extensão, hooks, testes e OKF.

## Manutenção e recuperação

Reinstalar o hook:

```sh
.spec-kit/bin/install-hooks.sh
```

O instalador preserva hooks diferentes e recusa `core.hooksPath` já configurado.
Nesses casos, integre `.spec-kit/hooks/post-commit` manualmente.

Atualizar a extensão depois de editar sua fonte:

```sh
specify extension add --dev .spec-kit/extensions/okf --force
```

Validar tudo:

```sh
npm test --prefix .spec-kit
npm run validate --prefix .spec-kit
npm run audit --prefix .spec-kit
specify integration list
specify extension list
```

Recuperação de sync:

- sem recibo: execute `$speckit-okf-sync`;
- commit da Wiki falhou: corrija e execute `auto-sync-wiki.js` novamente;
- código/testes, spec, plano ou tarefas mudou: refaça converge antes do sync;
- somente Wiki mudou após revisão: refaça `$speckit-okf-sync`;
- trava residual: confira o PID no arquivo antes de removê-lo;
- feature incorreta: use `feature.js select`;
- skills ausentes: abra o Codex na raiz e reinicie a sessão.

## Referências

- [Guia rápido](QUICKSTART.md)
- [Fluxo avançado e exemplo complexo](ADVANCED-WORKFLOW.md)
- [Regras do agente](.spec-kit/system-rules.md)
- [Constituição](.specify/memory/constitution.md)
- [Índice da Wiki](.knowledge/index.md)
- [Roteiro do sync](.spec-kit/templates/wiki-sync-prompt.md)
- [Guia técnico](.spec-kit/README.md)

Referências externas: [GitHub Spec Kit](https://github.com/github/spec-kit),
[integração Codex](https://github.github.io/spec-kit/reference/integrations.html)
e [OKF v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md).


## Autoridade e descoberta seletiva

| Artefato | Autoridade |
| --- | --- |
| Constitution | Princípios de desenvolvimento invariáveis |
| OKF | Conhecimento do sistema existente, sustentado por fontes |
| spec.md | Comportamento desejado, what/why |
| plan.md | Integração técnica, decisões e Knowledge Context |
| tasks.md | Execução e rastreabilidade |
| Código e testes | Realidade executável e evidências |
| Converge | Avaliação de aderência, com revisão real do agente |

Consulte índice → conceitos afetados → dependências relevantes. Cerca de 5–10
conceitos costuma ser um início útil, sem limite rígido e sem carregar todo o
bundle. Conhecimento stale, sem fonte ou contraditório requer descoberta dirigida.
OKF referencia contratos formais; não substitui OpenAPI/Protobuf.

## Execução por fases e Spec of Specs

```text
$speckit-implement Implemente somente T001–T005. Pare depois.
Execute as verificações do lote no plano. Não continue se falharem.
```

Registre evidências e, quando apropriado, faça commit local do lote. Em seguida,
execute o próximo intervalo. Só depois do gate final completo execute converge;
enquanto houver gaps, repita o ciclo sem atualizar OKF. Status verified indica
qualidade final aprovada; completed exige convergência e Wiki sincronizada.

Tente fases antes de decompor uma feature. Se sub-specs independentes forem
necessárias, mantenha um roadmap em specs/backlog/<roadmap>/ com dependências,
contratos compartilhados e critérios de integração. O roadmap é coordenação,
não feature executável nem domínio OKF. Cada sub-spec tem seu ciclo completo e
apenas uma pode ficar ativa. Fatos futuros ficam nas specs, não na Wiki atual.
