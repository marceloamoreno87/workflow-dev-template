# Correção: {{FEATURE}}

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

## Problema e resultado esperado

[Bug, evidência, impacto e comportamento correto. Aprovação explícita obrigatória.]

## Cenário de regressão

```gherkin
Scenario: Reproduzir e corrigir o problema
  Given [condição]
  When [ação]
  Then [resultado correto]
```

`verified` registra qualidade final aprovada; `completed` exige converge e sync
da Wiki bem-sucedidos. Tipos/build não aplicáveis exigem `applicable: false`
e `reason` concreta prevista no plano, sem comando fictício.
