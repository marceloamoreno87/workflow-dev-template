# Codex model routing

The Harness selects model and reasoning effort explicitly for every Agent Thread. Routing changes only after representative evals.

| Work | Model | Effort |
|---|---|---:|
| Classification, summarization, indexing, mechanical updates | `gpt-5.6-luna` | `low` |
| Project exploration and context collection | `gpt-5.6-terra` | `low` |
| Product Spec and acceptance criteria | `gpt-5.6-terra` | `medium` |
| Routine implementation with closed Spec | `gpt-5.6-terra` | `medium` |
| Bounded tests and fixes | `gpt-5.6-terra` | `medium` |
| Standard QA and review | `gpt-5.6-terra` | `high` |
| Architecture and difficult modeling | `gpt-5.6-sol` | `high` |
| Ambiguous or cross-cutting implementation | `gpt-5.6-sol` | `medium` |
| Security review | `gpt-5.6-sol` | `high` |
| Critical review | `gpt-5.6-sol` | `high` |
| Persistent failure or exceptional decision | `gpt-5.6-sol` | `xhigh` |

## Routing rules

- Luna receives no write capability, privileged shell, or normative decision.
- Escalation follows Luna → Terra → Sol and records its reason.
- Escalation does not reset fix-cycle or cost budgets.
- Reviewer and Security never reuse the Implementer's Agent Thread.
- Model, effort, token use, cached tokens, duration, and cost are telemetry dimensions.
- End-to-end cost per accepted result matters more than price per token.

## Evals

The baseline covers Issue triage, Spec generation, a small implementation in each supported stack, a cross-cutting bug, test creation, code review, security review, and OKF retrieval. Promotion requires equal or better acceptance, escaped-defect rate, intervention count, duration, and cost within budget.
