# Isolate secrets in the system keyring

Local credentials are stored in the operating system keyring and referenced symbolically from configuration. The daemon resolves a secret only for the narrowly scoped operation that needs it, while persisted state, logs, prompts, agent context, and Project files must contain neither the value nor a recoverable representation of it.
