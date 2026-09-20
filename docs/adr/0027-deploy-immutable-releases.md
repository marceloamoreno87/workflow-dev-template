# Deploy immutable releases

Projects produce immutable artifacts identified by source commit, with GitHub Releases recording checksums, SBOMs, checks, and changelog metadata. Coolify deploys the immutable reference rather than rebuilding an old commit, while Feature exposure and data migrations retain independent lifecycle and rollback policies.
