# Feature flag provider options

## Decision context

The Development Harness needs a self-hosted feature flag provider that can run on Coolify, integrate through OpenFeature, target users or clients in the only deployed environment, support Go, Python, and Next.js, expose safe defaults and kill switches, and preserve an auditable path back to Git. The system is initially operated by one developer, so operational simplicity matters more than enterprise collaboration features.

## Shortlist

### GO Feature Flag

GO Feature Flag is OpenFeature-native, MIT-licensed, self-hosted, and centered on a stateless relay proxy reading declarative flag configuration. Its official documentation lists providers for Go, Python, Node.js, browser JavaScript/TypeScript, React, and additional languages; it supports targeting, progressive rollouts, in-process or remote evaluation, GitHub and other configuration stores, and exporting exposure events to OpenTelemetry and other sinks.

Strengths for this Harness:

- smallest operational footprint among the full candidates;
- configuration-first design aligns with Git as source of truth;
- first-class OpenFeature and OFREP integration;
- required stacks are covered;
- in-process evaluation reduces runtime dependency on the relay proxy;
- exposure tracking can feed the Harness telemetry model.

Trade-offs:

- less of a collaborative control-plane UI than Unleash or Flagsmith;
- governance and approval need to be supplied by GitHub and the Harness;
- runtime configuration management is intentionally narrower than a database-backed feature-management platform.

Primary sources: [official documentation](https://gofeatureflag.org/docs), [OpenFeature support](https://gofeatureflag.org/product/open-feature), [Python provider](https://gofeatureflag.org/docs/sdk/server_providers/openfeature_python), [usage tracking](https://gofeatureflag.org/docs/tracking/flag-usage-tracking).

### Flipt

Flipt is self-hosted and Git-oriented, with official OpenFeature providers documented for Node, browser, Go, Python, and other languages. Its platform supports targeting, several storage backends, OpenTelemetry, audit-event sinks, and container deployment. Flipt v2 emphasizes self-hosted Git-native operation; some advanced SCM, security, and governance functions are part of Flipt Pro.

Strengths for this Harness:

- strong alignment with Git-native flag definitions;
- broader management UI and operational model than a bare relay proxy;
- all required application languages are documented;
- audit events and OpenTelemetry integration fit the Harness observability model.

Trade-offs:

- more moving parts than GO Feature Flag;
- v1 and v2 documentation and capabilities must be evaluated carefully during adoption;
- some advanced Git workflow and governance features are commercial.

Primary sources: [OpenFeature integrations](https://docs.flipt.io/v1/integration/openfeature), [self-hosted architecture](https://docs.flipt.io/v1/usecases/cloudnative), [audit events](https://docs.flipt.io/v1/configuration/auditing/overview), [Flipt v2 Pro boundaries](https://docs.flipt.io/v2/pro).

### Unleash

Unleash is a mature feature-management control plane with self-hosting, local SDK evaluation, targeting, metrics, and governance capabilities. Official OpenFeature providers are documented for Python and Node.js, but the exact supported-provider matrix for every required stack should be confirmed in a spike before selection.

Strengths for this Harness:

- mature operational UI and feature-management model;
- strong targeting and rollout concepts;
- local evaluation and caching in the documented OpenFeature providers.

Trade-offs:

- heavier control plane for a single operator;
- OpenFeature coverage across Go, Python, and browser/Next.js needs explicit validation;
- potentially more platform than the v1 requires.

Primary sources: [official documentation](https://docs.getunleash.io/), [Python OpenFeature provider](https://docs.getunleash.io/sdks/openfeature/python), [Node OpenFeature provider](https://github.com/Unleash/unleash-openfeature-node-provider).

### Flagsmith

Flagsmith provides an open-source feature-management platform with self-hosting, targeting, OpenFeature providers, and broad language SDK coverage. Its open-source edition includes core flags, segments, identities, and feature management, while audit logs, change requests, SAML, and advanced access controls are positioned as enterprise features.

Strengths for this Harness:

- broad SDK coverage and a mature UI;
- user identity and segment targeting fit client acceptance in production;
- supports self-hosting and OpenFeature.

Trade-offs:

- the governance capabilities most relevant to this design may require the paid tier;
- heavier deployment and operation than a stateless relay;
- overlaps with approval and audit functions already owned by GitHub and the Harness.

Primary sources: [official documentation](https://docs.flagsmith.com/), [OpenFeature](https://www.flagsmith.com/openfeature), [self-hosting and edition boundaries](https://www.flagsmith.com/self-hosted), [open-source capabilities](https://www.flagsmith.com/open-source).

### flagd

flagd is the reference OpenFeature flag daemon. It can read JSON or YAML definitions from files, HTTP, object stores, gRPC, or Kubernetes and supports targeting through JsonLogic-compatible rules. It is a runtime evaluator and synchronization component rather than a complete management control plane.

Strengths for this Harness:

- closest alignment with the OpenFeature specification;
- lightweight and suitable for declarative Git-managed flags;
- useful baseline for conformance and failure-mode tests.

Trade-offs:

- no complete workflow UI, approval system, or product-level audit model;
- the Harness would need to build more lifecycle and operational tooling itself;
- better suited as infrastructure building block than the default product for this design.

Primary sources: [installation](https://flagd.dev/installation/), [sync configuration](https://flagd.dev/reference/sync-configuration/), [provider targeting](https://flagd.dev/reference/specifications/providers/), [flag definitions](https://flagd.dev/reference/flag-definitions/).

## Recommendation

Use **GO Feature Flag** for the first vertical slice and retain **Flipt** as the strongest alternative. GO Feature Flag best matches the current priorities: one operator, Git-backed truth, OpenFeature-first integration, all three required stacks, exposure telemetry, low operational weight, and self-hosting on Coolify.

Before accepting it permanently, run a spike that proves:

1. targeted enablement for a single client identity in Go, Python, and Next.js;
2. safe defaults during relay unavailability;
3. config update and rollback through a reviewed Git change;
4. exposure events exported into OpenTelemetry;
5. a kill switch that takes effect within the required operational window;
6. backup and restoration of all flag declarations;
7. no leakage of targeting rules or unrelated flags to browser clients;
8. clean deployment and health checking on Coolify.

If the spike reveals that interactive operations and richer UI are more valuable than minimal infrastructure, repeat the same fixture against Flipt before changing the application-facing OpenFeature contract.
