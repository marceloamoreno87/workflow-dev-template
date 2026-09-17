# Plano: {{FEATURE}}

## Knowledge Context

Navegue pelo índice e leia apenas conceitos/dependências pertinentes.

| Conceito / caminho | Fontes e validade | Restrição existente | Decisão / requisito |
| --- | --- | --- | --- |
| [arquivo OKF] | [status, verified, stale_after, sources avaliados] | [fato observado] | [RF/SC e decisão técnica] |

### Lacunas e hipóteses

[Conhecimento ausente/obsoleto/conflitante, descoberta dirigida e evidência
necessária. Não descreva mudança futura como fato atual.]

## Mapeamento de impacto

Leia o manifesto e os conceitos envolvidos. Liste arquivos exatos de código,
testes, contratos, ADRs e Wiki que poderão mudar, incluindo exclusões/renomes.

| Domínio OKF | Dependências | Arquivos permitidos | Impacto |
| --- | --- | --- | --- |
| [id] | [ids] | [caminhos] | [mudança de comportamento] |

<!-- spec-kit:domains -->
```json
[]
```

Preencha o array acima; cada entrada tem este formato (exemplo ilustrativo):

```json
{
  "id": "pedidos",
  "path": "domains/pedidos.md",
  "depends_on": [],
  "source_paths": ["src/pedidos/"],
  "summary": "Descreva aqui o comportamento efetivamente implementado e seus contratos."
}
```

`source_paths` aceita arquivos exatos ou diretórios terminados em `/`, sem glob.
`path` é relativo a `.knowledge/`. `depends_on` referencia IDs do manifesto ou
deste ciclo. O resumo é contexto; revise a Wiki a partir da diff completa.

## Technical Context — Escolhas técnicas

[Stack confirmada, solução, decisões justificadas, alternativas e trade-offs.]

## Constitution Check

[Confirme aprovação, leitura da Wiki, TDD, escopo de arquivos e qualidade.]

## Project Structure

[Árvore concreta de código, testes, contratos e documentos da feature atual.]

## Estratégia de teste

[Mapeie RF/cenário → teste → arquivo. Inclua falhas, integração e regressões.]

- Testes e cobertura: [comando reproduzível]
- Lint sem avisos: [comando]
- Tipos estritos: [comando, se aplicável à stack]
- Build de produção: [comando, ou não aplicável com justificativa]
- Aceitação: [comando que verifica cenários RF/US; pode usar a suíte acima]
- Evidência esperada da fase vermelha: [falha correspondente ao requisito]

## Sequência e recuperação

[Ordem de execução, compatibilidade, migrações e reversão quando aplicáveis.]


## Fases e critérios de parada

[Intervalos de IDs/fases independentes, dependências, comandos por lote e gate
final completo. Pare após o lote solicitado ou falha de verificação.]

Converge avalia todo o trabalho executável antes da tarefa final da Wiki.
Alterar spec/plano/tarefas/código/testes após convergir exige nova avaliação.
Para features grandes, tente fases primeiro; use Spec of Specs apenas quando
sub-specs independentes forem necessárias, com roadmap e contratos compartilhados.
