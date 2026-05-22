# Start RingCentral connectors with user-owned credentials

RingCentral GitLab, Jira, and Wiki connectors should initially use user-owned connector credentials rather than workspace-owned service credentials. AI Desk's existing integrations are personal PAT-based, and preserving the delegated user's identity keeps external authorization, auditing, and revocation aligned with the user's actual access in GitLab, Jira, and Wiki.

The rejected alternative was to introduce a workspace-wide RingCentral service credential first. That would simplify shared setup but blur who authorized agent access, make external audit trails harder to explain, and risk granting agents broader access than the task's user actually has.
