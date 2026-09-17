# Spec: {{FEATURE}}

<!-- spec-kit:metadata -->
```json
{
  "status": "draft",
  "approved_by": null,
  "base_commit": null,
  "implementation_commit": null,
  "tests_passed": false,
  "quality": {
    "tests": {"command": null, "passed": false},
    "coverage": {"command": null, "percent": null, "passed": false},
    "lint": {"command": null, "passed": false},
    "types": {"command": null, "passed": false},
    "build": {"command": null, "passed": false},
    "acceptance": {"command": null, "passed": false}
  }
}
```

Estados: `draft` → `approved` → `verified` → `completed`. Aprovação deve identificar a pessoa.
Registre SHA completo e resultado real dos testes ao concluir a implementação.

## Contexto

[Problema de negócio, atores, fluxo atual e conceitos OKF consultados.]

## Objetivos

[Resultado esperado, critérios mensuráveis e itens fora do escopo.]

## Requisitos funcionais

- RF-001: [Comportamento observável, prioridade e critério de aceitação.]

## Contratos de interface

[Entradas, saídas, erros, autenticação, compatibilidade e links em .knowledge/interfaces/.]

## User Scenarios & Testing — Cenários de aceitação

### US1 — [Jornada principal] (Priority: P1)

[Valor de negócio e teste independente. Adicione US2/P2 conforme necessário.]

```gherkin
Feature: {{FEATURE}}
  Scenario: Caminho principal de RF-001
    Given [estado inicial]
    When [ação do usuário]
    Then [resultado observável]

  Scenario: Entrada inválida
    Given [estado inicial]
    When [entrada inválida]
    Then [erro esperado sem efeitos indevidos]
```

## Restrições e dúvidas

[Qualidade, limites, hipóteses e decisões ainda necessárias.]

## Success Criteria

- SC-001: [Resultado mensurável, verificável sem impor tecnologia.]

## Key Entities

[Entidades de negócio, quando aplicável.]

## Assumptions

[Hipóteses explicitadas e dependências confirmadas pela Wiki.]

`verified` registra qualidade final aprovada; `completed` exige converge e sync
da Wiki bem-sucedidos. Tipos/build não aplicáveis exigem `applicable: false`
e `reason` concreta prevista no plano, sem comando fictício.
