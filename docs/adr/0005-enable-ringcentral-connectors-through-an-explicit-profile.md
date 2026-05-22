# Enable RingCentral connectors through an explicit profile

RingCentral GitLab, Jira, and Wiki support should be delivered as optional connector adapters behind a RingCentral Connector Profile, not as default public Multica integrations. This preserves Multica's generic Connector, Connector Credential, Project Resource, and Connector Adapter model while allowing RingCentral deployments to expose a RingCentral Section in integration settings.

The rejected alternative was to add RingCentral GitLab, Jira, and Wiki as always-visible integration cards. That would leak one company's private service assumptions into the public product surface and make later connector providers look like exceptions instead of normal adapters.
