# Validate RingCentral credentials before save

RingCentral connector credentials should be validated before saving. If validation fails, the raw token is rejected and not stored. If validation succeeds, the Connector Credential Vault stores encrypted ciphertext plus provider, owner, workspace, credential status, upstream identity metadata, and `last_validated_at`.

The settings UI shows configured, valid, invalid, or never-validated status and upstream identity metadata without returning raw token values. Manual validation is supported. During agent connector commands, upstream authentication or authorization failures mark the credential invalid and return a recoverable task error that tells the delegated user to reconfigure the credential.

The rejected alternative was to save tokens first and run periodic background validation. Periodic validation would create unnecessary traffic against internal RingCentral services and still would not guarantee a credential remains valid at task execution time.
