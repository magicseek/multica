# Store RingCentral credentials in an encrypted vault

RingCentral connector credentials should be stored in a server-side encrypted Connector Credential Vault when the RingCentral Connector Profile is enabled. The vault stores ciphertext plus ownership, provider, workspace, validation status, and timestamps. Raw PAT values are accepted only during save or validation, are never returned to the frontend, and are never written into task context, `.multica/project/resources.json`, agent custom environment variables, or agent prompt material.

Connector commands call the Multica server. The server checks the task's Connector Delegated User, decrypts that user's credential, and invokes the RingCentral connector adapter. Enabling the RingCentral Connector Profile must require a configured credential encryption key or KMS; otherwise the profile should fail closed at startup.

The rejected alternative was to reuse `custom_env` or place tokens in the daemon/agent environment. `custom_env` is documented as plaintext server storage, and injecting RingCentral PATs into task runtime context would make least-privilege enforcement and auditability depend on agent behavior rather than server authorization.
