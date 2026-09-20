# Require structured Agent Role results

Every Agent Role receives a bounded envelope containing its objective, authorized context, capabilities, data classification, budget, deadline, and completion criteria. It returns a JSON-Schema-validated result containing outcomes, changes, evidence, blockers, usage, Knowledge Proposals, and privileged action requests; prose may accompany the result, but the daemon never advances workflow from unvalidated free text.
