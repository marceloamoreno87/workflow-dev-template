# Fluxo avançado do Spec Kit com Wiki OKF

Este guia demonstra o uso avançado do workflow Spec Kit + Wiki OKF em uma
feature complexa, tanto em um projeto novo quanto em um sistema existente. O
exemplo é hipotético e permanece tecnologicamente neutro: em projeto novo, a
stack deve ser escolhida a partir dos requisitos; em projeto existente, deve ser
confirmada no repositório e no ambiente real antes de planejar a mudança.

## Dois caminhos de entrada

Os dois tipos de projeto convergem para o mesmo ciclo, mas não partem da mesma
baseline:

```text
Projeto novo ───────→ princípios e requisitos iniciais ──┐
                                                        ├─→ ciclo da feature
Projeto existente ─→ descoberta dirigida do legado ─────┘
```

### Caminho A — projeto novo

Antes da primeira feature:

1. revise a constituição e os princípios de qualidade;
2. identifique objetivos de negócio, atores, fluxos, restrições e escala;
3. mantenha stack e arquitetura em aberto enquanto os requisitos não as
   justificarem;
4. permita que o primeiro plano defina a estrutura mínima, comandos de qualidade
   e primeiros domínios;
5. alimente a Wiki progressivamente por meio das features concluídas.

Exemplo de entrada:

```text
$speckit-specify Quero criar o primeiro fluxo de pagamento do produto...
```

### Caminho B — projeto existente

Antes da primeira feature, faça uma descoberta limitada aos domínios e
dependências que a mudança alcançará:

1. leia o manifesto, o índice e os conceitos OKF já existentes;
2. inspecione stack, módulos, contratos, persistência e integrações reais;
3. execute verificações não destrutivas para confirmar comandos de qualidade;
4. diferencie comportamento coberto por testes, observado no código, descrito em
   documentação e ainda hipotético;
5. registre fontes e validade do conhecimento sem inventar `verified`;
6. planeje testes de caracterização para comportamentos legados sem proteção.

Exemplo de entrada:

```text
Antes de especificar, faça a descoberta do módulo atual de pedidos e pagamentos.
Mapeie somente os caminhos, contratos, integrações e testes afetados por esta
mudança. Registre lacunas e hipóteses sem tratá-las como fatos.

$speckit-specify Quero adicionar idempotência ao pagamento existente...
```

Não é obrigatório documentar todo o legado antes de começar. A baseline pode
crescer por fatias: contexto afetado → feature → sync → próximo contexto. O
`$speckit-okf-sync` cobre a diff a partir do `base_commit`; não transforma todo o
sistema anterior em conhecimento confiável automaticamente.

### Ponto de convergência

Depois da baseline inicial, os dois caminhos usam os mesmos controles:

- spec em `draft` e aprovação explícita;
- plano com domínios, arquivos permitidos e estratégia de testes;
- tarefas TDD e uma única feature ativa;
- evidências reais de testes, cobertura, lint e tipos;
- convergência entre requisitos, artefatos e código;
- sincronização semântica da Wiki e arquivamento.

## Feature de exemplo

> Permitir que clientes paguem pedidos por um provedor externo, com
> idempotência, processamento de webhooks, reembolso e auditoria.

O fluxo completo cria pontos de controle entre requisitos, decisões técnicas,
implementação, validação e atualização do conhecimento:

```text
Contexto OKF
    ↓
Specify → Clarify → Aprovação
    ↓
Plan → Tasks → Analyze
    ↓
Implementação TDD
    ↓
Converge ── encontrou lacunas? ──→ Implement novamente
    ↓
Sync da Wiki
    ↓
Archive
```

## 1. Criar a especificação

Inicie o ciclo descrevendo a necessidade, os comportamentos esperados, os itens
fora do escopo e as dúvidas conhecidas:

```text
$speckit-specify

Quero permitir pagamentos de pedidos por um provedor externo.

A feature deve:
- criar uma tentativa de pagamento para um pedido;
- impedir cobranças duplicadas;
- receber atualizações por webhook;
- suportar pagamento aprovado, recusado, expirado e estornado;
- permitir reembolso total;
- manter trilha de auditoria;
- continuar consistente se o provedor repetir ou entregar eventos fora de ordem.

Fora do escopo:
- pagamento parcelado;
- múltiplos provedores;
- reembolso parcial;
- conciliação financeira manual.

Ainda precisamos decidir:
- volume esperado;
- prazo máximo de confirmação;
- política de retry;
- retenção dos dados;
- requisitos de compliance.
```

A spec é criada no backlog e selecionada em `.specify/feature.json`. Seu status
inicial é `draft`; nenhuma implementação pode começar nesse estado.

### Requisitos observáveis

Uma boa especificação descreve comportamento sem impor prematuramente banco,
fila ou framework:

```text
RF-001: Criar uma tentativa de pagamento para um pedido elegível.

RF-002: Repetir a mesma solicitação com a mesma chave de idempotência
não pode gerar uma segunda cobrança.

RF-003: Processar notificações duplicadas sem repetir efeitos de negócio.

RF-004: Uma notificação antiga não pode regredir o estado final de um
pagamento.

RF-005: Registrar quem solicitou um reembolso, quando e por qual motivo.

RF-006: Não armazenar dados sensíveis do cartão.
```

### Cenários de aceitação

```gherkin
Scenario: Repetição da criação do pagamento
  Given que uma tentativa foi criada com a chave "abc"
  When o cliente repete a solicitação com a chave "abc"
  Then recebe o resultado da tentativa existente
  And nenhuma nova cobrança é enviada ao provedor

Scenario: Webhook duplicado
  Given que o evento "evt-123" já foi processado
  When o provedor envia novamente "evt-123"
  Then o evento é reconhecido
  And nenhum efeito de negócio é repetido

Scenario: Evento fora de ordem
  Given que o pagamento já está aprovado
  When chega uma notificação anterior indicando processamento
  Then o pagamento permanece aprovado
  And a ocorrência fica registrada para auditoria
```

Critérios mensuráveis podem incluir ausência de duplicação sob concorrência,
prazo máximo para reconhecer webhooks, recuperação de eventos não processados,
cobertura integral dos cenários críticos e cobertura global mínima definida no
manifesto.

## 2. Esclarecer decisões arquiteturalmente relevantes

Para uma feature complexa, execute:

```text
$speckit-clarify
```

As perguntas devem atacar ambiguidades que alterariam arquitetura, escopo ou
risco. Por exemplo:

1. A confirmação pode demorar segundos, minutos ou horas?
2. O sistema deve continuar aceitando pedidos quando o provedor estiver fora?
3. Qual é o pico esperado de criações e webhooks por segundo?
4. Pagamentos aprovados podem ser revertidos por eventos posteriores?
5. Por quanto tempo os registros de auditoria devem ser retidos?

Respostas hipotéticas:

```text
- Pico: 50 criações/s e 200 webhooks/s.
- Confirmação normal em até 2 minutos.
- Disponibilidade alvo: 99,9%.
- Webhooks podem ser duplicados e entregues fora de ordem.
- Auditoria retida por 5 anos.
- Criação pode retornar "pendente" quando o provedor demorar.
```

As respostas são incorporadas à `spec.md`; não devem permanecer somente na
conversa.

## 3. Aprovar explicitamente

Depois de revisar a especificação:

```text
Aprovado. $speckit-plan
```

Essa declaração aprova a spec atual e solicita o plano. A metadata passa de
`draft` para `approved` e identifica quem concedeu a aprovação. Aprovar a spec
significa aceitar comportamento e escopo, não antecipar todas as decisões
técnicas do planejamento.

## 4. Gerar o plano técnico

O planejamento consulta o manifesto, o índice e os conceitos pertinentes da
Wiki, além de contratos, ADRs, `.specify/feature.json` e a spec aprovada:

```text
$speckit-plan
```

Em projeto novo, o plano define a stack mínima e cria os primeiros limites de
domínio. Em projeto existente, ele preserva a stack confirmada por padrão e
explicita compatibilidade, migração e impacto sobre comportamento legado. Uma
troca de tecnologia exige um requisito ou benefício concreto e análise de risco.

Uma arquitetura proporcional pode começar com módulos no mesmo sistema, sem
adotar microserviços antes de haver necessidade demonstrável:

```text
API de pagamentos
       │
       ▼
Módulo de pagamentos ──────── Provedor externo
       │
       ├── persistência de tentativas e idempotência
       │
       └── caixa de saída para efeitos assíncronos

Endpoint de webhook
       │
       ▼
Inbox de eventos → validação → máquina de estados → auditoria
```

### Decisões importantes

- **Idempotência:** chave única composta pelo cliente, operação e chave
  recebida.
- **Webhooks:** identificador único de evento para reconhecer duplicatas.
- **Ordenação:** máquina de estados com transições válidas, em vez de aceitar
  simplesmente a última mensagem recebida.
- **Efeitos assíncronos:** padrão outbox quando existir risco de persistir o
  pagamento e perder a publicação do evento.
- **Retry:** somente para falhas transitórias, com backoff e limite.
- **Segurança:** validação da assinatura do webhook e rotação de segredo.
- **Privacidade:** armazenamento de identificadores do provedor, nunca de dados
  brutos do cartão.
- **Auditoria:** registro das transições e ações administrativas.
- **Recuperação:** mecanismo para eventos não processados, caso os requisitos e
  a stack justifiquem a complexidade.

O plano também delimita os arquivos permitidos e os domínios OKF afetados:

| Domínio OKF | Dependências | Arquivos previstos | Impacto |
| --- | --- | --- | --- |
| `pedidos` | — | `src/pedidos/` e testes | Elegibilidade e vínculo do pagamento |
| `pagamentos` | `pedidos` | `src/pagamentos/`, testes e contrato | Novo ciclo de pagamento |
| `auditoria` | `pagamentos` | `src/auditoria/` e testes | Registro de transições e reembolsos |

O bloco `spec-kit:domains` do plano declara esses domínios e orienta a futura
sincronização da Wiki. Os caminhos acima são apenas ilustrativos; os caminhos
reais dependem da estrutura aprovada no plano.

## 5. Gerar tarefas TDD

```text
$speckit-tasks
```

Essa etapa ativa a feature em `specs/active/` e registra o commit-base. Deve
existir exatamente uma feature ativa.

Um conjunto de tarefas poderia ser organizado assim:

```text
US1 — Criação idempotente
T001 Criar testes de aceitação para criação e repetição concorrente.
T002 Implementar modelo de tentativa e regra de idempotência.
T003 Integrar criação com o contrato do provedor.

US2 — Atualização por webhook
T004 Criar testes para assinatura inválida, duplicação e evento fora de ordem.
T005 Implementar inbox e deduplicação.
T006 Implementar máquina de estados.
T007 Registrar transições na auditoria.

US3 — Reembolso
T008 Criar testes de autorização, repetição e falha transitória.
T009 Implementar reembolso total idempotente.
T010 Expor o contrato e os erros da operação.

Resiliência
T011 Testar timeout e indisponibilidade do provedor.
T012 Implementar retry, backoff e recuperação.
T013 Testar concorrência e condições de corrida.

Validação
T014 Executar suíte, cobertura, lint e tipos.
T015 Registrar evidências reais em spec.md e tasks.md.
T016 Executar $speckit-okf-sync.
```

Cada tarefa deve apontar arquivos e requisitos específicos. Se surgir a
necessidade de alterar outro módulo, primeiro atualize o plano e as tarefas. Uma
ampliação material de escopo requer decisão do desenvolvedor.

## 6. Analisar consistência antes de implementar

```text
$speckit-analyze
```

Essa análise não altera código. Ela procura problemas como:

- requisito sem tarefa;
- cenário crítico sem teste;
- arquivo presente nas tarefas, mas ausente do plano;
- decisão arquitetural contraditória;
- reembolso descrito na spec, mas ausente do plano;
- auditoria exigida sem modelo ou teste correspondente;
- infraestrutura introduzida sem requisito que a justifique.

Exemplo de lacuna:

```text
RF-004 exige tolerância a eventos fora de ordem, mas nenhuma tarefa testa
duas entregas concorrentes com estados diferentes.
```

A correção seria atualizar plano e tarefas para explicitar a precedência dos
estados e criar o teste correspondente antes da implementação.

## 7. Implementar com TDD verificável

```text
$speckit-implement
```

Cada comportamento segue o ciclo:

```text
teste que falha pelo motivo esperado
    ↓
implementação mínima
    ↓
teste verde
    ↓
refatoração
    ↓
suíte de regressão
```

Em um módulo legado sem testes suficientes, acrescente antes um teste de
caracterização que confirme o comportamento atual. Esse teste não declara que o
comportamento está correto; apenas protege contra regressão acidental. O novo
comportamento continua seguindo vermelho → verde → refatoração.

As evidências registradas em `spec.md` e `tasks.md` devem ser resultados reais:

```text
Vermelho:
comando: npm test -- payment-idempotency
resultado: falhou porque duas cobranças foram criadas
data: ...

Verde:
comando: npm test -- payment-idempotency
resultado: 8 testes aprovados
data: ...

Qualidade:
testes: ...
cobertura: 87,4%
lint: zero avisos
tipos: aprovado em modo estrito
```

Os comandos também são ilustrativos e devem ser substituídos pelos comandos
reais definidos no plano. Para esta feature, a estratégia deve cobrir:

- transições da máquina de estados;
- restrição de idempotência;
- contrato com o provedor;
- assinatura do webhook;
- eventos duplicados e fora de ordem;
- concorrência;
- timeout e retry;
- autorização de reembolso;
- regressão do fluxo de pedidos.

## 8. Tratar descobertas durante a implementação

Se a implementação revelar que um pedido pode envolver moedas diferentes, por
exemplo, isso altera o modelo de negócio e não deve ser tratado como uma pequena
correção local.

O procedimento é:

1. interromper a parte afetada;
2. registrar a descoberta na spec;
3. decidir se múltiplas moedas entram nesta feature;
4. atualizar plano e tarefas;
5. obter nova confirmação se houver ampliação material;
6. continuar a implementação.

Uma escolha interna que não altere comportamento nem escopo pode apenas ser
registrada no plano.

## 9. Verificar convergência

Depois da primeira implementação:

```text
$speckit-converge
```

A skill compara código, spec, plano e tarefas. Se encontrar trabalho faltante,
acrescenta novas tarefas, por exemplo:

```text
T017 Criar teste de recuperação de evento após falha entre persistência e
processamento.

T018 Documentar o erro de conflito de idempotência no contrato público.
```

Nesse caso, repita:

```text
$speckit-implement
$speckit-converge
```

O ciclo continua até não haver divergências. `converge` não marca lacunas como
concluídas; ele as transforma em tarefas rastreáveis.

## 10. Sincronizar a Wiki OKF

A tarefa final de conhecimento executa:

```text
$speckit-okf-sync
```

O Codex analisa a diff completa entre o commit-base e a implementação final. A
Wiki poderia receber conceitos como:

```text
.knowledge/domains/pagamentos.md
.knowledge/interfaces/payment-provider.md
.knowledge/architecture/payment-state-machine.md
```

O conhecimento registra o que foi efetivamente implementado:

- responsabilidade do domínio;
- estados e transições válidas;
- invariantes de idempotência;
- contrato do webhook;
- regras de duplicação e ordenação;
- dependência com pedidos;
- limitações atuais;
- fontes e proveniência.

O sync também atualiza o índice e a versão do conhecimento, associa a revisão
ao commit, gera `wiki-sync.json`, valida YAML e hashes e cria um commit local
somente da Wiki. Se o código mudar depois, a revisão precisa ser refeita.

Em projeto novo, esse processo inaugura ou expande os conceitos do sistema. Em
projeto existente, atualiza o conhecimento do domínio revisado e pode incorporar
descobertas do legado somente quando elas tiverem fontes e validação suficientes.
Partes do sistema não analisadas permanecem fora do alcance do sync.

## 11. Arquivar o ciclo

```text
$speckit-okf-archive
```

O archive verifica:

- spec aprovada e concluída;
- tarefas encerradas;
- evidências de testes, cobertura, lint e tipos;
- convergência entre artefatos e código;
- Wiki sincronizada com o commit correto;
- recibo de sincronização válido.

Depois, move a feature para `specs/archive/` e cria um commit local apenas dos
artefatos da spec. O fluxo não faz push.

## Sequência prática completa

```text
$speckit-specify Quero implementar pagamentos externos...
$speckit-clarify

Aprovado. $speckit-plan

$speckit-tasks
$speckit-analyze
$speckit-implement
$speckit-converge
```

Se a convergência acrescentar tarefas:

```text
$speckit-implement
$speckit-converge
```

Quando tudo estiver convergente:

```text
$speckit-okf-sync
$speckit-okf-archive
```

## Rastreabilidade esperada

O objetivo do fluxo avançado é manter uma cadeia verificável:

```text
requisito → cenário → decisão → tarefa → teste → código → evidência → conhecimento
```

Cada decisão arquitetural deve atender a um requisito, cada requisito deve ter
validação e a Wiki deve refletir o comportamento efetivamente entregue, não
apenas a intenção inicial do plano.
