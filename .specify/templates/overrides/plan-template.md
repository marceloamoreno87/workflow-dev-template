# Plano: {{FEATURE}}

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
- Evidência esperada da fase vermelha: [falha correspondente ao requisito]

## Sequência e recuperação

[Ordem de execução, compatibilidade, migrações e reversão quando aplicáveis.]
