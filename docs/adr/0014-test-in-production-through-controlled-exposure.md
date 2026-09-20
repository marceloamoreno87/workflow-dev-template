# Test in production through controlled exposure

Production is the only deployed Acceptance Environment, so every unaccepted Feature must use an explicit Exposure Strategy such as targeted OpenFeature-compatible flags, shadow mode, dry-run, allowlisted side effects, or progressive rollout. A Feature cannot enter Client QA in production when its effects cannot be safely bounded. Active rollout flags require a safe default, kill switch, audit trail, owner, and removal within 14 days after full rollout.
