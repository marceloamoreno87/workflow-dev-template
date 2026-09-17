# Regras do workflow Spec Kit + Wiki OKF

Este projeto usa o Spec Kit oficial 1.0.6 para Codex, instalado em `.specify/`
e `.agents/skills/`. `.spec-kit/` contém a extensão local OKF e seus utilitários.

## Regras obrigatórias

No repositório template, preserve o estado inicial: sem features, conceitos
compilados, recibos ou constituição ratificada. Manutenção do template não deve
gerar conhecimento sobre o próprio workflow em `.knowledge/`. Valide com
`npm run audit:template --prefix .spec-kit`. Nos projetos consumidores, gere a
constituição e configure o manifesto antes da primeira feature; então aplique
o ciclo completo abaixo, incluindo sync da Wiki ao concluir.

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
   Novas specs podem entrar no backlog, mas não substituem o contexto ativo.
8. Antes do sync, registre em `spec.md` comandos e resultados estruturados para
   testes, cobertura, lint, tipos, build e aceitação. Cobertura deve atingir o mínimo do manifesto.

## Comandos reais no Codex

Digite `$` para selecionar as skills. Se não aparecerem, abra uma nova sessão
na raiz do repositório. Os antigos `/specify`, `/tasks` e `/archive` não existem;
o `/plan` nativo é o modo de planejamento do Codex, não esta etapa do Spec Kit.

| Skill | Resultado |
| --- | --- |
| `$speckit-constitution` | Gera os princípios do projeto; a constituição inicial é um template. |
| `$speckit-specify descrição` | Cria spec; hook OKF a coloca em backlog e preserva contexto. |
| `$speckit-clarify` | Esclarece requisitos pendentes. |
| `$speckit-plan` | Gera plano da spec aprovada, com domínios OKF e estratégia de testes. |
| `$speckit-tasks` | Gera tarefas TDD; hook ativa a feature e registra o commit base. |
| `$speckit-analyze` | Verifica coerência entre artefatos antes de implementar. |
| `$speckit-implement` | Executa o lote TDD solicitado; after_implement verifica, sem sync. |
| `$speckit-converge` | Avalia aderência; after_converge só sincroniza se converged. |
| `$speckit-okf-sync` | Revisa semanticamente o código e sincroniza a Wiki. |
| `$speckit-okf-archive` | Confirma convergência e Wiki atualizada; move para archive. |

## Aprovação e ciclo

`draft` → aprovação explícita → `approved` → qualidade final → `verified`
→ converge sem gaps → sync bem-sucedido → `completed`.
`base_commit` é registrado ao ativar e preservado até o fim da feature.
`implementation_commit` é preenchido ao
revisar a Wiki. Depois de revisão/commit, o script marca a tarefa da Wiki.

Implement respeita o intervalo de IDs/fase solicitado e deixa a tarefa da Wiki
pendente. after_implement executa verify, registra resultados e para em falha.
Depois da verificação final, execute implement → converge até resultado
converged. A tarefa da Wiki é encerramento diferido, não gap de comportamento;
não a duplique nas fases acrescentadas pelo core converge.

Somente after_converge com resultado converged registra convergence-report.md
e convergence.json e executa a skill de sync. O script não avalia aderência:
vincula a declaração de revisão real ao código/testes, spec, plano, tarefas e
relatório. Mudanças nesses elementos invalidam a convergência. O sync e archive
validam esse recibo. Não edite recibos manualmente nem invente resultados.
Ao concluir testes, pode criar commit local dos arquivos previstos na feature,
preservando staged alheio e sem push. Se já houver Wiki sincronizada e recibos
válidos, apenas valide, sem incrementar versão ou regenerar conhecimento.

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
after_converge, somente quando convergido. Commits `docs(wiki):` e a variável de execução interna impedem
recursão. Uma trava impede dois syncs simultâneos.

Falha de commit preserva os arquivos e o recibo; corrija o problema e execute
novamente `auto-sync-wiki.js`. Não recrie a revisão se seu conteúdo continua
válido. Não declare o ciclo concluído enquanto o sync estiver pendente.
O archive cria um commit local exclusivo dos artefatos da spec e preserva staged
alheio. Ele não faz push.


## Autoridades e contexto seletivo

Constitution governa; OKF descreve o sistema existente; spec é intenção; plan é
design; tasks organiza execução; código é realidade e testes são evidências.
OKF não substitui governança ou contratos formais. Divergências exigem revisão
explícita, sem converter intenção futura em conhecimento atual.

Leia índice → conceitos afetados → dependências/links/contratos relevantes.
Comece com cerca de 5–10 conceitos quando suficiente, ampliando por necessidade.
Avalie status, verified, stale_after e sources antes de confiar. No plan, preencha
Knowledge Context e vincule conceitos/restrições → decisões → RF/SC → tarefas.
Lacunas exigem descoberta dirigida, não redescoberta de todo o sistema.

Use clarify quando houver ambiguidade relevante, checklist para qualidade de
requisitos e analyze antes de implement para alterações com várias etapas ou
risco. Corrija findings críticos antes do código. Para trabalho grande, tente
fases verificáveis primeiro; depois, se necessário, Spec of Specs com roadmap,
sub-specs independentes e contratos compartilhados. Cada sub-spec percorre o
ciclo completo e mantém apenas uma feature ativa.
