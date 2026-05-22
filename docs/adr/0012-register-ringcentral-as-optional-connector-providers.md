# Register RingCentral as optional connector providers

RingCentral GitLab, Jira, and Wiki should be registered as optional Connector Providers in the generic connector framework, not modeled with RingCentral-specific top-level settings or credential tables. The connector registry exposes providers available for the current deployment profile, workspace connector rows record workspace-level enablement and provider configuration, connector credential rows store encrypted user-owned credentials, and project resource rows scope what an agent task may use.

The rejected alternative was to mirror AI Desk with dedicated RingCentral Jira, GitLab, and Wiki settings tables. That would make one company's private services first-class product schema, make later providers exceptions, and weaken the goal that RingCentral code remains pluggable and absent from public Multica deployments unless the RingCentral Connector Profile registers it.
