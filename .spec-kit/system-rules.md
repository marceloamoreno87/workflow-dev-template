# Regras do workflow Spec Kit + Wiki OKF

Este projeto usa o Spec Kit oficial 1.0.6 para Codex, instalado em `.specify/`
e `.agents/skills/`. `.spec-kit/` contém a extensão local OKF e seus utilitários.

## Regras obrigatórias

1. Antes de propor código ou planejar, leia `.knowledge/manifest.yaml`,
   `.knowledge/index.md` e conceitos dos domínios/dependências pertinentes.
2. Nenhuma alteração em `src/` sem spec aprovada, plano e tarefas preenchidos em
   `specs/active/`. Registre aprovação já dada na conversa, sem pedi-la novamente.
3. TDD: teste que falha pelo motivo esperado → implementação → testes verdes →
   refatoração. Registre comandos/resultados reais. Obedeça cobertura, lint e
   tipos definidos no plano e no manifesto.
4. Altere somente arquivos previstos no plano/tarefas. Atualize esses artefatos
   antes de ampliar o conjunto de arquivos; ampliação de escopo exige decisão do
   desenvolvedor. Não escolha stack de aplicação sem requisitos.
5. Os overrides oficiais em `.specify/templates/overrides/` são os templates
   canônicos do fluxo completo. Preserve seus blocos JSON e IDs de tarefas.
6. A feature atual é `.specify/feature.json`. Use `feature.js` para mover ou
   selecionar specs, mantendo esse contexto. Não se baseie apenas na branch Git.
7. Mantenha exatamente uma feature em `specs/active/`. A ativação exige `src/`
   e `.knowledge/` limpos para que a base delimite o ciclo sem contaminação.
8. Antes do sync, registre em `spec.md` comandos e resultados estruturados para
   testes, cobertura, lint e tipos. Cobertura deve atingir o mínimo do manifesto.

## Comandos reais no Codex

Digite `$` para selecionar as skills. Se não aparecerem, abra uma nova sessão
na raiz do repositório. Os antigos `/specify`, `/tasks` e `/archive` não existem;
o `/plan` nativo é o modo de planejamento do Codex, não esta etapa do Spec Kit.

| Skill | Resultado |
| --- | --- |
| `$speckit-constitution` | Atualiza princípios; a constituição inicial já está preenchida. |
| `$speckit-specify descrição` | Cria spec; hook OKF a coloca em backlog e preserva contexto. |
| `$speckit-clarify` | Esclarece requisitos pendentes. |
| `$speckit-plan` | Gera plano da spec aprovada, com domínios OKF e estratégia de testes. |
| `$speckit-tasks` | Gera tarefas TDD; hook ativa a feature e registra o commit base. |
| `$speckit-analyze` | Verifica coerência entre artefatos antes de implementar. |
| `$speckit-implement` | Executa tarefas; o hook final chama `$speckit-okf-sync`. |
| `$speckit-converge` | Avalia aderência ao spec/plano e acrescenta trabalho restante. |
| `$speckit-okf-sync` | Revisa semanticamente o código e sincroniza a Wiki. |
| `$speckit-okf-archive` | Confirma convergência e Wiki atualizada; move para archive. |

## Aprovação e ciclo

`draft` → aprovação explícita → `approved` → implementação/testes → `completed`.
`base_commit` é registrado ao ativar e preservado até o fim da feature.
`implementation_commit` é preenchido ao
revisar a Wiki. Depois de revisão/commit, o script marca a tarefa da Wiki.

O agente deve concluir a tarefa da Wiki executando a skill de sync antes de
declarar a implementação concluída. Esse passo pode rodar tanto pela tarefa
final quanto pelo hook after_implement; se já estiver sincronizado e não houver
mudanças, apenas valide o recibo, sem regenerar ou incrementar a versão.
Ao concluir testes, pode criar o commit local da implementação dentro do escopo
da feature para permitir o sync; preserve staged alheio e não faça push.

Fast-track mantém aprovação e TDD, com documentos curtos. Execute
`feature.js activate` antes de implementar, mesmo que a pasta já esteja active.
Specs arquivadas são histórico: para nova mudança, abra outro ciclo.

## Wiki OKF v0.2

- `.knowledge/index.md` declara `okf_version: "0.2"`. Todo conceito Markdown
  não reservado tem frontmatter YAML com `type` não vazio.
- O manifesto é uma extensão local para sistema, domínios e qualidade. YAML
  comum é suportado; não há exigência de JSON em linha para `domains`.
- O Codex lê a diff completa da base da feature até o commit final, confronta
  código e conceitos e atualiza responsabilidades, invariantes e contratos.
  O summary do plano é contexto, não substitui análise do código.
- Depois da revisão, o sincronizador atualiza `generated`, a fonte
  `implementation`, o índice e `knowledge_version`. Preserva campos desconhecidos
  e fontes anteriores. Cada domínio afetado precisa ter mudança semântica real.
  Ao alterar um conceito, remova `verified` antigo; só uma verificação posterior
  e comprovável pode recriá-lo.
- `wiki-sync.json` na spec vincula a revisão ao commit, base, plano e hashes dos
  documentos. O commit automático inclui somente conteúdo da Wiki coberto pelo
  recibo. O recibo comprova correspondência de bytes, não correção semântica.
- Valide o bundle com `node .spec-kit/bin/validate-okf.js`.
- Audite a instalação inteira com `npm run audit --prefix .spec-kit`.
- Use `.spec-kit/templates/wiki-sync-prompt.md` para o procedimento completo.

## Automação Git local

O post-commit tenta publicar uma revisão já pronta para o commit atual. Sem
revisão, registra pendência em `git rev-parse --git-path okf-auto-sync.log`.
O hook não inicia sessões de IA: a análise ocorre na sessão atual do Codex via
after_implement. Commits `docs(wiki):` e a variável de execução interna impedem
recursão. Uma trava impede dois syncs simultâneos.

Falha de commit preserva os arquivos e o recibo; corrija o problema e execute
novamente `auto-sync-wiki.js`. Não recrie a revisão se seu conteúdo continua
válido. Não declare o ciclo concluído enquanto o sync estiver pendente.
O archive cria um commit local exclusivo dos artefatos da spec e preserva staged
alheio. Ele não faz push.
