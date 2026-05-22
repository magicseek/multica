# Multica Domain Context

Multica coordinates human and AI work in a shared workspace. This context records product language that should stay stable across planning, UI, and architecture documents.

## Language

**Workspace**:
A team boundary that owns issues, projects, agents, skills, chats, and repositories.

**Repository**:
A code working target that an agent can read or write, whether it is backed by a remote Git URL, a user-selected local directory, or an agent-created local work directory.
_Avoid_: Codebase

**Repository Binding**:
A machine- or runtime-scoped authorization that maps a repository to an actual local path or daemon-managed work directory.
_Avoid_: Global path, shared local path

**Project**:
A planning container for related issues, similar to a Linear project or Jira epic.
_Avoid_: Repository, Codebase

**Chat Session**:
A private, creator-owned multi-turn conversation between one user and one agent.
_Avoid_: Team chat, shared project conversation

**Project-Associated Chat Session**:
A private **Chat Session** grouped under a **Project** for organization without changing who can see it.
_Avoid_: Project chat

**Chat-Originated Issue**:
An **Issue** created as an output of a **Chat Session**.
_Avoid_: Project issue by implication

**Chat Issue Proposal**:
A set of candidate issues proposed inside a **Chat Session** and awaiting user approval.
_Avoid_: Auto-created issues

**Chat Issue Proposal Item**:
One candidate issue inside a **Chat Issue Proposal**.
_Avoid_: Draft issue

**Proposal Artifact**:
A structured output from a chat agent that records a **Chat Issue Proposal** separately from prose.
_Avoid_: Markdown issue list

**Chat Plan Run**:
A stateful planning exchange inside a **Chat Session** that turns an idea into reviewable **Chat Issue Proposals** before execution.
_Avoid_: Plan-mode chat session, per-message plan toggle

**Plan Engine**:
A server-owned planning preset that defines how a **Chat Plan Run** gathers context, challenges assumptions, and decides when to propose issues.
_Avoid_: Workspace Skill, local runtime skill

**Plan Summary**:
A concise record of the confirmed requirements, rejected options, consensus notes, and remaining questions from a **Chat Plan Run**.
_Avoid_: Full workflow artifact, markdown-only recap

**Chat Plan Consultation**:
A bounded agent-to-agent consultation inside a squad-backed **Chat Plan Run**.
_Avoid_: Roundtable chat, sidecar agent, background helper

**Squad Plan Consensus**:
The lead agent's synthesized planning result after considering bounded **Chat Plan Consultations** from squad members.
_Avoid_: unanimous vote, infinite debate

**Lead Agent**:
The agent selected to bootstrap, coordinate, or own work when a repository starts without an existing Git remote or user-selected local directory.

**Output Metadata**:
Privacy-minimized facts about local or agent-managed task outputs, such as file names, relative paths, sizes, MIME types, and creation times, excluding file contents, diffs, logs, stack traces, screenshots, and absolute local paths.

**Connector**:
A workspace- or user-authorized connection to an external system that agents may use through scoped project resources.
_Avoid_: Integration, auth provider, project resource

**Connector Provider**:
A stable provider identity, such as GitHub, RingCentral GitLab, RingCentral Jira, or RingCentral Wiki, that can be registered into the connector framework.
_Avoid_: Integration card, provider-specific table

**Connector Provider Registry**:
The server-side registry that exposes available connector providers for the current deployment profile.
_Avoid_: Global hardcoded integrations list

**Workspace Connector**:
A workspace-level enablement and configuration record for one connector provider.
_Avoid_: User token, project resource

**Connector Adapter**:
A provider-specific implementation that translates a connector into external API calls without becoming the product domain model.
_Avoid_: Direct service import, token store, domain model

**Connector Capability**:
A named operation that an agent may perform through a connector adapter when the current task has a matching project-scoped resource and credential authority.
_Avoid_: Full API access, token permission, prompt instruction

**Connector Command**:
A Multica-controlled CLI command that invokes a Connector Capability through server-side authorization and provider adapter logic.
_Avoid_: Raw curl, MCP-only tool, direct token exposure

**Task-scoped Connector Command**:
A connector command invoked from an agent task and authorized by the task's agent identity, task identity, delegated user, and attached project resources.
_Avoid_: User-global CLI access, agent-local token use, runtime-inferred authority

**Connector Capability Audit Event**:
An audit record for one connector capability invocation, including task, agent, delegated user, provider, capability, resource, and result metadata.
_Avoid_: Prompt transcript, raw external response, secret log

**Semantic Task Branch**:
A Git branch created for one task using an intent prefix such as `feat/`, `fix/`, `refactor/`, `docs/`, `test/`, `chore/`, or `perf/`, plus issue/task identifiers and a short slug.
_Avoid_: Product-name prefix, default branch, shared long-lived branch

**Remote Write Policy**:
A workspace connector setting that determines which remote write capabilities are allowed for a provider.
_Avoid_: Token scope, agent preference, prompt instruction

**Git Auth Broker**:
A future daemon/server mediation layer that would let a local git process authenticate to a connector-backed repository without exposing raw connector credentials to the agent.
_Avoid_: PAT in git remote URL, PAT in environment, direct credential helper injection

**Connector Endpoint Configuration**:
Deployment- or profile-level base URL configuration for a connector provider's upstream API and web links.
_Avoid_: User preference, raw provider URL in task context, hardcoded public product URL

**Connector Credential Status**:
The validation state of a connector credential, such as `valid`, `invalid`, or `never_validated`, with upstream identity metadata and validation timestamps.
_Avoid_: Token truth, permission guarantee, background health check

**Connector Credential**:
A workspace- or user-owned secret reference used by a connector without copying raw secret values into task context.
_Avoid_: Raw token, permission snapshot

**Connector Credential Vault**:
The server-side encrypted storage boundary for connector credentials in deployments that explicitly enable credential-bearing connector profiles.
_Avoid_: custom_env, agent environment, project resource JSON

**User-owned Connector Credential**:
A connector credential controlled by one user and usable only under that user's delegated authority.
_Avoid_: Workspace token, service account

**Connector Delegated User**:
The human user whose connector credentials authorize connector operations for one queued agent task.
_Avoid_: Agent assignee, runtime owner, workspace owner by default

**Project Resource**:
A project-scoped pointer to external or local context, such as a repository, Jira project, Wiki space, or external document.
_Avoid_: Integration, workspace setting, copied content

**Container Project Resource**:
A project resource that bounds discovery or operations inside an external container, such as a GitLab repository, Jira project, or Wiki space.
_Avoid_: Company-wide scope, free search

**Pinned Project Resource**:
A project resource that points at a specific external item, such as a Jira issue or Wiki page, for task context.
_Avoid_: Search scope, copied content

**RingCentral Connector Profile**:
An explicit deployment profile that enables RingCentral-specific connector adapters and RingCentral settings UI.
_Avoid_: Default public integration, CLI profile, user profile

**Connector Profile Registration**:
The server startup step that conditionally registers connector providers, adapters, routes, resource validators, runtime guidance, and settings UI metadata for enabled connector profiles.
_Avoid_: Always-on import side effect, public integration default

**RingCentral Section**:
The settings section shown only under the RingCentral Connector Profile for configuring RingCentral GitLab, Jira, and Wiki connectors.
_Avoid_: Always-visible public settings, generic integration page

**Project Resource Management UI**:
The project-level UI for searching, attaching, reviewing, and removing Project Resources.
_Avoid_: Connector credential settings, provider admin panel, task prompt editor

**Security-first Connector Implementation Sequence**:
The implementation order that establishes profile gating, credential storage, delegated authority, and resource scoping before exposing agent-facing connector commands.
_Avoid_: UI-first connector rollout, adapter-first shortcut, token-first spike

**Connector Acceptance Gate**:
The minimum verification suite required before RingCentral connector work can be considered complete.
_Avoid_: Happy-path demo, manual-only test, unchecked token handling

## Relationships

- A **Workspace** contains zero or more **Repositories**
- A **Workspace** contains zero or more **Projects**
- A **Workspace** contains zero or more **Chat Sessions**
- A **Workspace** may have zero or more **Workspace Connectors**
- A **Workspace Connector** enables one **Connector Provider** for one **Workspace**
- A **Workspace Connector** may expose read-only status derived from **Connector Endpoint Configuration**
- The **Connector Provider Registry** exposes only providers available to the current deployment profile
- **Connector Profile Registration** determines which providers appear in the **Connector Provider Registry**
- A **Connector Provider** may have one **Connector Adapter**
- A **Connector** exposes zero or more **Connector Capabilities**
- Agents invoke **Connector Capabilities** through **Connector Commands** first; MCP wrappers may be added later over the same backend capability model
- Agent-initiated connector operations use **Task-scoped Connector Commands**
- A **Task-scoped Connector Command** requires a valid agent identity, task identity, workspace match, active task, **Connector Delegated User**, matching **Project Resource**, and authorized **Connector Credential**
- Each **Task-scoped Connector Command** produces a **Connector Capability Audit Event**
- A **Connector** uses one or more **Connector Credentials**
- A **Connector Credential Vault** stores raw connector secrets only as encrypted ciphertext plus ownership and validation metadata
- A **Connector Credential** has a **Connector Credential Status** derived from save-time validation, manual validation, or upstream auth failures
- RingCentral GitLab, Jira, and Wiki connectors initially use **User-owned Connector Credentials**
- An agent task that uses user-owned connector credentials captures a **Connector Delegated User** when the task is queued
- Retried or rerun agent tasks inherit the original **Connector Delegated User** unless a user explicitly re-authorizes the run
- Comment-triggered agent tasks use the comment author as the **Connector Delegated User** when the author is a human member
- Agent-authored comment triggers inherit the parent task's **Connector Delegated User** rather than granting the agent new connector authority
- Chat tasks use the chat session creator or triggering human chat message author as the **Connector Delegated User**
- Autopilot tasks require an explicitly configured **Connector Delegated User** before using user-owned connector credentials
- A **Project** may have zero or more **Project Resources**
- A **Project Resource** may reference a **Connector** and narrows what a **Project** can use
- A **Project Resource** may be a **Container Project Resource** or **Pinned Project Resource**
- Agents may use connector-backed external data only through **Project Resources** attached to the current **Project**
- Agents may use RingCentral **Connector Capabilities** only when the current task has a matching **Project Resource** and **Connector Delegated User**
- RingCentral GitLab repositories are represented as `ringcentral_gitlab_repo` **Container Project Resources** with GitLab project identity, path, web URL, and default branch metadata
- RingCentral GitLab write operations use **Semantic Task Branches** rather than product-name prefixes such as `multica/`
- A **Semantic Task Branch** must not target a default or protected branch
- RingCentral GitLab write operations obey the workspace connector's **Remote Write Policy**
- RingCentral Jira projects are represented as `ringcentral_jira_project` **Container Project Resources** for bounded search, while Jira issues are represented as `ringcentral_jira_issue` **Pinned Project Resources** for specific context
- RingCentral Wiki spaces are represented as `ringcentral_wiki_space` **Container Project Resources** for bounded search, while Wiki pages are represented as `ringcentral_wiki_page` **Pinned Project Resources** for specific context
- RingCentral connector commands default to search and read only inside attached **Container Project Resources**; specific external context should be attached as **Pinned Project Resources**
- RingCentral GitLab MVP capabilities include token validation, project access validation, repository tree/file reads, branch and merge request listing, branch creation, commits/pushes, and merge request creation
- RingCentral GitLab MVP write capabilities allow branch creation, commits/pushes, and merge request creation only for attached repositories and task-scoped **Semantic Task Branches**
- RingCentral GitLab MVP file reads, branch creation, commits, pushes, and merge request creation are server-mediated API operations, not local git operations authenticated with a raw PAT
- RingCentral GitLab MVP write capabilities do not merge merge requests, delete branches, modify project settings, or modify protected branch rules
- When the GitLab **Remote Write Policy** disables ordinary remote writes, the adapter rejects generic branch writes and may allow only explicitly whitelisted merge-request preparation operations
- Full local checkout and test workflows continue to use Multica **Repository** and **Repository Binding** flows; RingCentral connector credentials are not injected into local git for MVP
- A **Git Auth Broker** may later allow task-scoped private GitLab clone/push without exposing raw RingCentral PATs to agents
- RingCentral Jira MVP capabilities include token validation, project listing, issue search, issue retrieval, and read-only Jira issue context attachment
- RingCentral Wiki MVP capabilities include token validation, page or space search, page retrieval, and read-only Wiki page context attachment
- RingCentral Jira and Wiki MVP capabilities do not write comments, status, pages, or spaces
- RingCentral connector adapters do not grant agents unbounded company-wide exploration outside attached **Project Resources**
- RingCentral MVP is CLI-first so every supported runtime can use connector capabilities through Multica-controlled commands; MCP is optional and provider-specific
- RingCentral **User-owned Connector Credentials** are stored in the **Connector Credential Vault**, not in `custom_env`, task context, `.multica/project/resources.json`, or agent environment variables
- RingCentral **User-owned Connector Credentials** are validated before save; invalid tokens are rejected rather than stored
- RingCentral credential status stores upstream identity metadata and `last_validated_at` when validation succeeds
- RingCentral settings UI shows configured/valid/invalid/never_validated status and upstream identity without returning raw token values
- RingCentral connector commands mark credentials invalid when upstream returns authentication or authorization failures, and tasks surface recoverable reconfiguration errors
- RingCentral MVP does not run periodic background validation across all credentials
- A **Connector Command** calls the Multica server, and the server resolves the task's **Connector Delegated User**, decrypts that user's credential, and invokes the RingCentral **Connector Adapter**
- RingCentral agent operations are allowed only through **Task-scoped Connector Commands**, not through user-global CLI commands or agent-visible raw tokens
- Human UI and CLI flows may configure credentials, validate credentials, search external resources, and attach **Project Resources**, but they do not bypass task-scoped authorization for agent capability execution
- The **RingCentral Connector Profile** requires a configured credential encryption key or KMS before startup can expose RingCentral credential storage
- If a required **User-owned Connector Credential** is missing, the task waits for explicit user configuration or authorization instead of falling back to a workspace-wide secret
- The **RingCentral Section** is visible only when the **RingCentral Connector Profile** is enabled
- The **RingCentral Section** appears under workspace integration settings, but each member configures their own **User-owned Connector Credentials** there
- The **RingCentral Section** contains provider cards for RingCentral GitLab, Jira, and Wiki with endpoint status, workspace connector status, current-user credential status, and token save/test/delete controls
- Workspace admins may control RingCentral connector availability and see aggregate credential readiness, but they may not view or edit another member's **User-owned Connector Credentials**
- Workspace admins may configure RingCentral GitLab **Remote Write Policy** from the **RingCentral Section**
- **Project Resource Management UI** lives on Project surfaces, not inside the **RingCentral Section**
- RingCentral GitLab, Jira, and Wiki are optional **Connector Providers** registered by the **RingCentral Connector Profile**, not RingCentral-specific top-level tables or public always-on integrations
- The **RingCentral Connector Profile** enables RingCentral GitLab, Jira, and Wiki **Connector Adapters** without making them default public Multica features
- RingCentral code lives in isolated connector packages and is registered only through **Connector Profile Registration** when the RingCentral profile is enabled at server startup
- The RingCentral connector MVP uses runtime profile gating and provider registration rather than Go build tags, plugin binaries, or a separate server binary
- RingCentral GitLab, Jira, and Wiki base URLs come from **Connector Endpoint Configuration** at deployment/profile startup, not from per-user settings
- RingCentral **Connector Endpoint Configuration** includes GitLab API base URL, GitLab web base URL, Jira base URL, and Wiki base URL
- RingCentral **Workspace Connectors** may display configured endpoints and status, but ordinary workspace admins do not edit base URLs in the MVP
- When the RingCentral profile is not enabled, RingCentral providers, routes, settings UI metadata, resource validators, and agent runtime guidance are absent
- Existing GitHub App integration remains on its current GitHub-specific installation, webhook, pull request mirror, and settings flows during the RingCentral connector MVP
- The generic connector framework may reserve a GitHub provider identity later, but the RingCentral connector MVP does not migrate GitHub storage or behavior
- RingCentral connector implementation follows the **Security-first Connector Implementation Sequence**
- The **Security-first Connector Implementation Sequence** starts with connector schema, registry, profile gating, and credential vault
- The **Security-first Connector Implementation Sequence** then adds task queue delegated-user capture before provider adapters, settings UI, project resource UI, task-scoped commands, audit logs, tests, and docs
- RingCentral connector delivery must pass the **Connector Acceptance Gate**
- The **Connector Acceptance Gate** verifies profile-disabled absence, credential vault secrecy, startup failure without encryption key, delegated-user capture, task-scoped command rejection cases, fake-provider adapter behavior, UI separation, and end-to-end connector smoke flows
- A **Project** may reference one or more **Repositories**
- A **Project** may group zero or more **Project-Associated Chat Sessions**
- A **Project-Associated Chat Session** remains visible only to its creator
- A **Project-Associated Chat Session** is discovered through **Projects** navigation and the owning **Project** detail page
- The **Projects** sidebar tree may show a small recent subset of **Project-Associated Chat Sessions** under each **Project**
- The **Projects** sidebar tree lists all active **Projects**, even when a **Project** has no recent **Project-Associated Chat Sessions**
- An expanded **Project** sidebar row shows at most three recent **Project-Associated Chat Sessions**
- A **Project-Associated Chat Session** may be started from the Project navigation row action or from the owning **Project** detail page
- A **Project-Associated Chat Session** remains as private history if its associated **Project** is archived or deleted
- A **Project-Associated Chat Session** stores a creation-time **Project** snapshot so deleted Projects can still be represented in chat history
- A loose **Chat Session** is discovered through the workspace **Chats** navigation entry
- A loose **Chat Session** may be started from the top-level New Chat action
- A **Chat Session** is opened as a page-level workspace route rather than a global floating window
- A **Chat Session** title may start from the first user message as a fallback
- A **Chat Session** title may be updated after the first agent execution using an agent-provided summary title
- A user-edited **Chat Session** title is not overwritten by an agent-provided summary title
- Archiving or deleting a **Chat Session** does not delete its **Chat-Originated Issues** or **Output Metadata**
- A **Chat Session** may contain zero or more **Chat Plan Runs**
- A **Chat Plan Run** belongs to exactly one **Chat Session**
- A **Chat Plan Run** uses exactly one **Plan Engine**
- A **Plan Engine** is selected from Multica-provided presets unless a later product decision opens user-defined engines
- A **Chat Plan Run** is stateful across multiple user replies, so a user does not reselect plan mode for every answer
- A **Chat Plan Run** may produce zero or more **Chat Issue Proposals**
- A **Chat Plan Run** may store one **Plan Summary**
- A **Plan Summary** explains the proposal context but does not create issues by itself
- A member still approves **Chat Issue Proposal Items** before any **Chat-Originated Issues** are created
- A squad-backed **Chat Plan Run** is led by the selected squad's lead agent
- A squad-backed **Chat Plan Run** may contain zero or more **Chat Plan Consultations**
- A **Chat Plan Consultation** is addressed to a squad member through chat mention syntax and returns to the lead agent through a lead mention
- A **Chat Plan Consultation** only targets agents in the selected squad roster
- A **Squad Plan Consensus** is lead-synthesized, not a requirement that every consulted squad member agrees
- The workspace sidebar shows only active **Chat Sessions** updated in the last five days as a quick-access tree
- The workspace sidebar may label loose **Chat Session** quick access as **Recents** while the full product concept remains **Chats**
- The **Recents** sidebar section lists loose **Chat Sessions** only, not **Project-Associated Chat Sessions**
- **Recents** initially shows active loose **Chat Sessions** updated in the last five days, then loads older active loose **Chat Sessions** in pages
- **Recents** groups loose **Chat Sessions** by recency labels such as today, yesterday, the last five days, and older
- **Recents** loads older loose **Chat Sessions** inline in the sidebar rather than navigating to the full **Chats** archive
- Primary workspace navigation groups **Inbox**, **My Issues**, **Issues**, **Projects**, **Agents**, **Squads**, **Autopilot**, **Usage**, and **Recents** without a separate **Workspace** heading
- **Runtimes**, **Workflows**, and **Skills** are **Configure** surfaces inside **Settings**, not primary workspace navigation entries
- **Settings** groups its middle navigation as **Account**, **Configure**, and **Workspace**
- The **Configure** settings group contains **Runtimes**, **Workflows**, and **Skills**
- The workspace sidebar exposes **Settings** as one fixed bottom entry; settings subsections are discovered inside the **Settings** page
- The workspace sidebar keeps **Settings** fixed at the bottom while the main navigation and expandable **Projects** and **Recents** lists scroll independently above it
- A **Project-Associated Chat Session** provides the default **Project** for new **Chat-Originated Issues**
- A **Chat-Originated Issue** keeps an explicit link to its source **Chat Session**
- A **Chat-Originated Issue** is created after explicit user approval from a **Chat Issue Proposal** or issue creation flow
- **Chat-Originated Issues** created from a **Chat Issue Proposal** default to backlog status
- A **Chat Issue Proposal** is persisted before approval
- A **Chat Issue Proposal** contains one or more **Chat Issue Proposal Items**
- A **Chat Session** may contain multiple **Chat Issue Proposals**
- A **Proposal Artifact** is the source of truth for a **Chat Issue Proposal**
- A **Proposal Artifact** uses a minimal versioned schema with `version` and `proposals`
- A **Chat Issue Proposal Item** may include issue draft fields such as `title`, `description`, optional `priority`, optional `labels`, and optional `assignee_id`
- A **Chat Issue Proposal** appears inline in the **Chat Session** conversation after the proposing agent message
- The **Chat Session** Issues view shows the same **Chat Issue Proposal** objects for review and follow-up management
- The **Chat Session** Issues view may present pending **Chat Issue Proposal Items** in a **Proposed** review lane before the normal **Backlog** issue lane
- The **Chat Session** Issues view orders its Kanban lanes as **Proposed**, **Backlog**, and then the normal issue workflow lanes
- A **Chat Issue Proposal Item** in the **Proposed** lane uses proposal review controls rather than normal issue drag-and-drop
- Approving a **Chat Issue Proposal Item** removes it from **Proposed** and creates a real **Issue** in **Backlog**
- Batch approval from the **Chat Session** Issues view defaults to all selected pending **Chat Issue Proposal Items** in that **Chat Session**, while preserving proposal grouping for context
- A **Chat Session** has `Chat`, `Issues`, and `Outputs` page tabs
- The **Chat Session** `Issues` tab count reflects created **Chat-Originated Issues**, not pending **Chat Issue Proposal Items**
- The **Chat Session** `Outputs` tab count reflects available **Output Metadata** records
- A **Chat Issue Proposal Item** may become zero or one **Chat-Originated Issue**
- A member may edit **Chat Issue Proposal Items** before approving issue creation
- A member may not edit the target **Project** of a **Chat Issue Proposal Item** before approval
- A **Chat Issue Proposal Item** keeps an approval-time snapshot of the content used to create its **Chat-Originated Issue**
- A partially approved **Chat Issue Proposal** records unapproved selected-out items as skipped
- A skipped **Chat Issue Proposal Item** can be restored to pending before later approval
- The approving member is the creator of a **Chat-Originated Issue**
- The proposing agent remains provenance for a **Chat Issue Proposal**, not the issue creator
- A **Chat Issue Proposal** records detailed provenance such as source chat message, source task, and proposing agent
- A **Chat-Originated Issue** keeps a lightweight origin link to the source **Chat Session**
- Approving multiple **Chat Issue Proposal Items** creates issues in one transaction; validation failure creates no partial batch
- A created **Chat Session** does not move between **Projects**
- **Chat-Originated Issues** follow normal workspace and project visibility
- **Output Metadata** from a **Chat Session** inherits the **Chat Session** privacy boundary until explicitly attached to a shared issue, project, or published artifact
- A **Chat Session** output view contains **Output Metadata**, not **Chat Issue Proposals**
- A **Chat Session** output view aggregates **Output Metadata** from the session's own chat tasks and from agent tasks on issues created from that session
- A chat may select or reference a **Repository**, but does not own it
- A **Repository** is a first-class workspace entity, not only JSON embedded in workspace settings
- A **Repository** may have zero or more **Repository Bindings**
- A **Lead Agent** may initialize a **Repository** when no user-selected local directory or remote Git URL exists
- A **Repository Binding** may store a local path, but the full path is visible only to the binding owner and the corresponding runtime or daemon by default
- When no Git remote or local directory is selected, the **Lead Agent** initializes a **Repository Binding** inside its own runtime's Multica-managed work area
- A **Repository** may move one way from local or agent-managed work to local Git, then to remote Git
- A from-scratch build request should create or select a **Repository** early instead of remaining a long-lived no-repository chat draft
- Local or agent-managed task outputs may expose **Output Metadata** to the cloud while keeping output contents local until explicitly published

## Example Dialogue

> **Dev:** "Should we create a new codebase object for from-scratch work?"
> **Domain expert:** "No. Reuse **Repository**. A repository may start as a local draft before it has any remote Git URL."

> **Dev:** "Should RingCentral GitLab be a normal integration card in every Multica workspace?"
> **Domain expert:** "No. It appears only when the **RingCentral Connector Profile** is enabled; public Multica exposes the generic connector framework."

> **Dev:** "If RingCentral credentials are user-owned, should they still be configured from workspace integration settings?"
> **Domain expert:** "Yes. The **RingCentral Section** is the discovery surface, but every member edits only their own **User-owned Connector Credentials**."

> **Dev:** "Should RingCentral Jira and Wiki support write operations in the first version?"
> **Domain expert:** "No. Keep Jira and Wiki read-only for MVP; GitLab may write only through project-scoped repository operations."

> **Dev:** "Should RingCentral connector access be MCP-only?"
> **Domain expert:** "No. Start with Multica **Connector Commands** so every runtime can use the same guarded capabilities; MCP can wrap those commands later."

> **Dev:** "Can RingCentral PATs reuse agent `custom_env`?"
> **Domain expert:** "No. Store them in the encrypted **Connector Credential Vault** and never inject raw tokens into agent runtime context."

> **Dev:** "Should RingCentral Jira, GitLab, and Wiki each get dedicated settings tables?"
> **Domain expert:** "No. Model them as optional **Connector Providers** registered into the generic connector framework."

> **Dev:** "Should a Project Resource point only to a whole RingCentral system, or to precise external objects?"
> **Domain expert:** "Use both: **Container Project Resources** bound search and operations, and **Pinned Project Resources** attach exact Jira issues or Wiki pages as context."

> **Dev:** "Can an agent use a normal user CLI command to call RingCentral connectors?"
> **Domain expert:** "No. Agent connector calls must be **Task-scoped Connector Commands** with task, delegated-user, resource, credential, and audit checks."

> **Dev:** "Should the first connector framework pass migrate the existing GitHub integration too?"
> **Domain expert:** "No. Keep GitHub on its existing GitHub App path for now; introduce RingCentral connectors in parallel."

> **Dev:** "Does RingCentral require a separate Multica binary or build tag in the first version?"
> **Domain expert:** "No. Use startup-time **Connector Profile Registration** and isolated packages first; add stronger build-time separation later only if needed."

> **Dev:** "Should RingCentral GitLab agent branches use a `multica/` prefix?"
> **Domain expert:** "No. Use semantic intent prefixes such as `feat/` or `fix/`, with task identifiers for traceability."

> **Dev:** "Should RingCentral GitLab PATs be used by local git checkout in the first version?"
> **Domain expert:** "No. Start with server-mediated GitLab API capabilities; local git credential brokering is a later design."

> **Dev:** "Should every user configure their own RingCentral Jira/GitLab/Wiki base URLs?"
> **Domain expert:** "No. Base URLs are **Connector Endpoint Configuration** supplied by the deployment or connector profile."

> **Dev:** "Should RingCentral tokens be saved before validation and checked later in the background?"
> **Domain expert:** "No. Validate on save, support manual test, and mark invalid on use-time auth failures; do not run periodic background validation in MVP."

> **Dev:** "Should RingCentral settings also be where users attach Jira issues, Wiki pages, and GitLab repos to projects?"
> **Domain expert:** "No. Settings is for provider and credential readiness; **Project Resource Management UI** handles project-scoped external context."

> **Dev:** "Should we build RingCentral UI first and backfill authorization later?"
> **Domain expert:** "No. Follow the **Security-first Connector Implementation Sequence** so storage, profile gating, delegated user capture, and resource scoping exist before agent commands."

> **Dev:** "Is a successful happy-path demo enough to ship RingCentral connectors?"
> **Domain expert:** "No. The **Connector Acceptance Gate** must prove absence when disabled, credential secrecy, delegated authority, task scoping, provider guardrails, and UI/resource separation."

## Flagged Ambiguities

- "Project" was used to mean both a planning container and a codebase. Resolved: **Project** remains the issue-planning container; **Repository** is the code working target.
- "Integration" can mean a settings page, an external connection, or provider code. Resolved: product domain uses **Connector**, project scoping uses **Project Resource**, and provider code uses **Connector Adapter**; UI copy may still label the settings area Integrations.
- "Profile" already appears as user profile and CLI profile language. Resolved: use **RingCentral Connector Profile** for the deployment opt-in that exposes RingCentral-specific connector surfaces.
- RingCentral could be implemented as dedicated Jira/GitLab/Wiki settings tables. Resolved: use generic **Workspace Connector**, **Connector Provider**, **Connector Credential**, and **Project Resource** contracts, with RingCentral providers registered only by the **RingCentral Connector Profile**.
- RingCentral code could require build tags, plugins, or a separate binary. Resolved: MVP uses startup-time **Connector Profile Registration** plus isolated packages; public deployments do not enable the RingCentral profile.
- RingCentral upstream URLs could be hardcoded or user-configurable. Resolved: use deployment/profile-level **Connector Endpoint Configuration**, visible as read-only workspace connector status in MVP.
- RingCentral token status could be discovered by periodic background validation. Resolved: validate before save, allow manual validation, and mark invalid on use-time upstream auth failures.
- GitHub already has a production GitHub App integration with webhook and PR mirror behavior. Resolved: do not migrate GitHub in the RingCentral connector MVP; leave existing GitHub tables, routes, and UI behavior intact.
- Project Resources could point at broad systems such as "RingCentral Jira" or only exact external objects. Resolved: use **Container Project Resources** for bounded discovery and **Pinned Project Resources** for exact external context.
- "Capability" can mean an external token scope or an agent action. Resolved: **Connector Capability** means an agent-exposed operation gated by Project Resource scope and delegated user authority, not the raw upstream token scope.
- MCP is not the first RingCentral connector surface because current Multica runtime support is provider-dependent. Resolved: MVP exposes **Connector Commands** first and may add MCP wrappers later.
- Multica public docs say the server does not see user API keys, while RingCentral PATs require server-mediated API calls. Resolved: this behavior is limited to explicit credential-bearing connector profiles such as the **RingCentral Connector Profile**, and startup requires configured credential encryption.
- Agent connector operations could rely on the daemon's current user token or global CLI config. Resolved: use **Task-scoped Connector Commands** that validate `X-Agent-ID`, `X-Task-ID`, active task state, **Connector Delegated User**, matching **Project Resource**, and credential ownership on every invocation.
- RingCentral GitLab task branches could use a product-name prefix such as `multica/`. Resolved: use **Semantic Task Branches** with conventional intent prefixes and task traceability instead.
- RingCentral GitLab could power local git checkout by injecting PATs into the daemon or agent environment. Resolved: MVP is server-mediated API-first; a task-scoped **Git Auth Broker** is a future extension.
- RingCentral tokens could be modeled as workspace-managed service credentials or copied from ai-desk user settings. Resolved: first-version RingCentral GitLab, Jira, and Wiki credentials are **User-owned Connector Credentials**.
- "Workspace integration settings" can imply workspace-owned secrets. Resolved: the **RingCentral Section** lives in workspace settings for discovery and provider administration, while each member manages only their own **User-owned Connector Credentials**.
- The RingCentral settings page could become a project context editor. Resolved: keep provider readiness and credentials in the **RingCentral Section**, and place project-scoped search/attach flows in **Project Resource Management UI**.
- Implementation could begin with UI or provider adapters before security boundaries are in place. Resolved: follow the **Security-first Connector Implementation Sequence**.
- RingCentral connector completion could be judged by manual happy-path testing. Resolved: require the **Connector Acceptance Gate** across profile gating, credential handling, delegated authority, command authorization, adapters, UI, and smoke flows.
- The user for connector access could be inferred at runtime from issue creator, assignee, commenter, or workspace owner. Resolved: each queued agent task captures a **Connector Delegated User** at queue time.
- RingCentral capabilities could copy every AI Desk operation immediately. Resolved: MVP keeps GitLab write-capable within attached project repositories, while Jira and Wiki are read-only context sources.
- "Project chat" can imply a shared team conversation. Resolved: use **Project-Associated Chat Session** for private chats grouped under a project.
- The primary **Chats** navigation entry can imply every chat in the workspace. Resolved: **Chats** is the total entry for loose **Chat Sessions**; **Project-Associated Chat Sessions** are found through **Projects** navigation and Project detail.
- **Recents** is a sidebar quick-access label for recent loose **Chat Sessions**, not a replacement term for **Chat Session** or the full **Chats** archive.
- The five-day **Recents** window is the initial quick-access window only; pagination can continue beyond five days through older active loose **Chat Sessions**.
- **Recents** date grouping is a presentation aid and does not create a new **Chat Session** category.
- The old **Workspace** sidebar section heading is removed; primary workspace navigation remains flat until expandable **Projects** and **Recents** sections.
- Existing direct URLs for **Runtimes**, **Workflows**, and **Skills** remain addressable, but sidebar discovery moves under **Settings**.
- Public product labels may diverge from canonical domain terms during a frontend rewrite. Resolved: preserve the existing terms in data model, API, and implementation contracts, and use a separate display vocabulary for user-facing navigation and copy.
- Older **Project-Associated Chat Sessions** are recovered through the owning **Project** detail `Chats` view, not through **Recents**.
- The owning **Project** detail `Chats` view remains the complete history surface for that **Project**'s **Project-Associated Chat Sessions**.
- When a **Project** has more associated chat history than the sidebar subset, the sidebar links to the owning **Project** detail `Chats` view for the full list.
- Project visibility in the **Projects** sidebar tree is based on active **Project** status, not on recent chat activity.
- Starting a **Project-Associated Chat Session** from different surfaces should not create different flows. Resolved: Project navigation row actions and Project detail `Chats` use the same new-chat route with the selected Project context, and the actual session is created when the first message is sent.
- Starting a loose **Chat Session** uses the top-level New Chat action and a new-chat route without Project context. The first send requires an explicit agent selection.
- The old global Chat floating action button and floating window are replaced by page-level Chat routes as the primary workspace chat experience.
- **Chat Session** titles should not repeat Project names because navigation already provides that hierarchy. The initial title may fall back to the first user message, then the first agent execution may provide a more accurate summary title.
- Title generation needs an explicit source or user-edited marker so agent-generated summary titles do not overwrite a title the user already changed.
- Agent-provided **Chat Session** summary titles travel through the same task completion metadata handoff as other structured chat outputs, and the backend applies them only when the user has not edited the title.
- A chat task may produce multiple structured outputs, such as summary title metadata, **Chat Issue Proposals**, and **Output Metadata**
- The daemon may read multiple fixed local manifests for structured chat outputs, but uploads them together in one task completion payload for backend processing
- The sidebar's five-day chat window is a quick-access filter only. Complete loose chat history remains available from **Chats**, and complete project-associated chat history remains available from the owning **Project** detail `Chats` view.
- If a **Project** is archived or deleted, its associated **Chat Sessions** are not shown under the active **Projects** sidebar tree. The **Chat Session** page shows an archived or deleted Project context chip, and already created issues keep their normal issue state and provenance.
- If the underlying **Project** row is deleted, a **Project-Associated Chat Session** does not become a loose **Chat Session**. It retains its creation-time Project snapshot for historical display.
- Data model should not infer loose chat status from `project_id IS NULL` alone. A deleted Project may also leave `project_id` null, so the session needs an explicit project association marker plus a creation-time Project snapshot.
- If a **Chat Session** is archived or deleted, its created issues and output metadata are retained. The session's issue provenance remains readable even if the conversation is no longer active.
- A **Project-Associated Chat Session** records the project context chosen at session creation, not a mutable label. **Chat-Originated Issues** keep their own source link.
- Issues merely sharing the same **Project** as a **Chat Session** are not **Chat-Originated Issues** unless the explicit source link is present.
- A **Chat Issue Proposal** is not an **Issue** until a user approves creation.
- A **Chat Issue Proposal Item** is not an **Issue** and should not appear on issue boards before approval.
- **Proposed** is a review lane for pending **Chat Issue Proposal Items** inside a **Chat Session** Issues view, not an **Issue** status.
- A Markdown checklist in a chat reply is not a **Chat Issue Proposal** unless it is backed by a **Proposal Artifact**.
- A **Proposal Artifact** does not let the proposing agent choose issue status. Created issues use backlog status from backend rules.
- **Plan mode** is a UI affordance for starting or continuing a **Chat Plan Run**, not a separate **Chat Session** type.
- A **Chat Plan Run** is not an **Issue**, and it is not the approval boundary for creating issues.
- A **Plan Engine** is not the same as a workspace **Skill**. The server may project engine instructions into local runtime context, but the cloud preset remains authoritative.
- A **Chat Plan Consultation** is not an ordinary issue comment mention. It routes through the **Chat Plan Run** and remains scoped to that chat's planning transcript.
- **Squad Plan Consensus** can complete with missing or failed consultation replies if the lead agent records the gap in the **Plan Summary**.
- **Chat Issue Proposal Item** Project assignment is derived from the source **Chat Session** context. Project reassignment happens later through normal issue editing, not inside proposal approval.
- Inline proposal controls in the chat transcript and proposal controls in the **Chat Session** Issues view operate on the same persisted **Chat Issue Proposal**, not duplicated draft state.
- Pending **Chat Issue Proposals** are presented inside the **Chat Session** `Issues` tab without increasing the created issue count.
- **Chat Issue Proposals** belong with issue creation workflow, not the **Output Metadata** view.
- The **Chat Session** output view is scoped to outputs produced by the chat and by issues that were explicitly created from that chat; unrelated issues in the same Project are excluded.
- The member approval action, not the agent proposal, is the authorization boundary for creating **Chat-Originated Issues**.
- Detailed proposal provenance belongs on **Chat Issue Proposal** records. The created **Issue** keeps only the lightweight source **Chat Session** origin link and can reach proposal/item details through the proposal tables.
- Batch approval for **Chat Issue Proposal Items** is atomic. If any selected item fails validation, no issues are created and the user edits the proposal before retrying.
- **Chat Issue Proposal Items** keep approval-time snapshots so later issue edits do not erase what was approved from the proposal.
- Partial approval sets the **Chat Issue Proposal** to a partially accepted state, marks unapproved items as skipped, and allows skipped items to be restored later from the **Chat Session** `Issues` tab.
- The **Chat Session** `Issues` tab groups by **Chat Issue Proposal**, shows pending proposals first, and then orders proposal groups by creation time.
- **Repository** ownership is workspace-level. **Project** and chat flows reference repositories but do not own them.
- **Repository** is a first-class entity. Workspace settings may expose repositories, but they are not only an embedded JSON list.
- Repository management belongs under **Workspace** settings rather than the primary workspace navigation.
- Work surfaces such as chat, project detail, and quick-create show lightweight repository chips or selectors even though full repository management lives in workspace settings.
- Project-level repository settings store references to **Repositories**, not copied remote URLs.
- Local paths are binding-private. Other workspace members may see binding availability and machine labels, but not absolute local paths by default.
- No-source repository startup is runtime-owned: the server records intent, and the selected lead agent's runtime creates the actual local working directory.
- Repository source transitions are one-way: `local_dir` or `agent_managed` may become `local_git`, and `local_git` may become `remote_git`.
- Build-from-scratch flows should establish a **Repository** before sustained agent work so chats, issues, and projects can share the same repository identity.
- Code-oriented **Projects** should automatically reference a primary **Repository**; ordinary planning **Projects** may have no repository.
- Cloud-visible **Output Metadata** must not include contents, diffs, detailed logs, screenshots, stack traces, or absolute local paths by default.

## Agent Workflows

This context defines how Multica describes reusable agent execution behavior across workspaces, projects, issues, and task runs.

## Language

**Workflow Definition**:
A workspace-level editable template that describes how an agent should execute a class of work.
_Avoid_: hardcoded execution protocol, agent prompt blob

**System-Seeded Workflow Definition**:
A read-only workflow definition provided by Multica as a built-in starting point inside a workspace.
_Avoid_: hardcoded built-in template, magic slug

**Workflow Fork**:
An editable user-owned copy of a workflow definition.
_Avoid_: editing the system seed directly

**Workflow Revision**:
A versioned schema for a workflow definition that can be drafted, published, deprecated, and snapshotted.
_Avoid_: silent in-place template mutation

**Workflow Draft Revision**:
An unpublished, mutable workflow revision used to hold edits before they are reviewed and published for future workflow runs.
_Avoid_: browser-only unsaved edits, mutating a published workflow in place

**Project Workflow Binding**:
The default workflow selected for issues that belong to a project.
_Avoid_: agent workflow, project prompt

**Issue Workflow Override**:
An explicit per-issue selection of a different workflow, usually to route a small bug, research task, or one-off task through a lighter process.
_Avoid_: skip workflow, bypass workflow

**Workflow Snapshot**:
The immutable workflow content captured for a specific agent task when the task is queued.
_Avoid_: live workflow reference during execution

**Workflow Run**:
A server-owned execution instance created from a workflow snapshot for one agent task.
_Avoid_: browser-driven flow, live workflow mutation

**Workflow Step Run**:
A server-owned execution state record for one step inside a workflow run.
_Avoid_: prompt-only step, UI-only step

**Workflow Step Run Status**:
A Multica runtime state for a workflow step run, such as pending, ready, running, waiting review, waiting quality, waiting manual, waiting external, paused, blocked, failed, completed, or skipped.
_Avoid_: ai-desk uppercase step status enum

**Workflow Step Execution Kind**:
The workflow schema's declaration of whether a step is completed by the assigned agent, by a human/manual action, or by an external actor.
_Avoid_: daemon runtime mode, ai-desk execution mode enum

**Workflow Step Contract**:
The user-facing editing contract for one workflow step: its title, purpose, expected completion output, required status, and review or quality expectations.
_Avoid_: exposing every workflow step schema field as the default editor

**Workflow Step Inspector**:
The focused editing panel for the currently selected workflow step, grouping step contract fields and advanced settings without expanding every step inline.
_Avoid_: per-step form stack, repeating every advanced tab in every step row

**Workflow Step Inspector Section**:
A user-intent grouping inside the Workflow Step Inspector, such as Basics, Instructions, Inputs, Outputs, or Checks.
_Avoid_: ai-desk field-name tabs, schema-shaped navigation

**Workflow Prompt Focus Editor**:
A modal editor for the selected workflow step's long-form agent prompt and checklist constraints, opened from the step inspector without leaving the current step.
_Avoid_: schema jump, full step modal, separate save model

**Workflow Step Completion Expectation**:
A user-facing description of how a workspace member or agent can tell that a workflow step is complete.
_Avoid_: executable completion condition, forcing every step to produce a named artifact

**Workflow Step Gate**:
The user-facing gate setting on a workflow step that summarizes whether the run continues immediately, waits for human approval, runs an agent quality check, or requires both.
_Avoid_: exposing review and quality gate internals as separate default step fields

**Manual Workflow Step**:
A workflow step that a human workspace member completes through Multica.
_Avoid_: unclaimable agent step

**External Workflow Step**:
A workflow step that waits for an actor or system outside first-version Multica workflow execution.
_Avoid_: hidden integration, fake agent step

**Workflow Execution Batch**:
A daemon execution pass that may process one or more ready workflow step runs while preserving step-level state.
_Avoid_: cold-start every step, hidden whole-workflow loop

**Workflow Input Request**:
A workflow-scoped request for a human answer that must be resolved before a waiting workflow step run can continue.
_Avoid_: clarification comment, issue status, ordinary mention, manual step

**Workflow Execution Command**:
A Multica CLI command used by an agent to read or update workflow run, step run, artifact, or quality state.
_Avoid_: first-version MCP-only workflow control

**Workflow Step Retry**:
Re-execution of a failed, rejected, blocked, or otherwise retryable workflow step run inside the same workflow run.
_Avoid_: hidden workflow rerun

**Workflow Rerun**:
A newly queued agent task that creates a new workflow snapshot and workflow run.
_Avoid_: mutating current run workflow content

**Workflow Artifact**:
A step-scoped deliverable produced or revised during a workflow run.
_Avoid_: terminal output, untracked file

**Reviewable Workflow Artifact**:
A workflow artifact whose content is intentionally stored by Multica for cloud review, approval, or dependent workflow steps.
_Avoid_: local output, output metadata, published local file

**Workflow Artifact Version**:
An immutable revision of a workflow artifact produced by initial execution or retry.
_Avoid_: overwritten artifact

**Workflow Artifact Diff**:
A review aid that compares two workflow artifact versions without replacing either complete version.
_Avoid_: artifact source of truth, patch-only artifact

**Workflow Artifact Input**:
A declared artifact that a workflow step can read or requires from earlier step output.
_Avoid_: implicit dependency edge

**Workflow Artifact Template**:
Schema-embedded instructions or files describing the expected shape of a workflow artifact.
_Avoid_: external catalog dependency in first-version migration

**Workflow Review**:
A human or agent decision on a workflow artifact before dependent step runs proceed.
_Avoid_: informal comment-only approval

**Workflow Quality Gate**:
A workflow-defined evaluation that records whether a workflow artifact satisfies the step's quality criteria.
_Avoid_: optional lint note, hidden rubric

**Workflow Quality Gate Result**:
A recorded quality evaluation outcome for a workflow artifact, including its producer and whether it is blocking.
_Avoid_: anonymous platform verdict

**Party Mode**:
An ai-desk workflow feature for simulated multi-role step discussion or review that Multica intentionally does not migrate.
_Avoid_: legacy party mode metadata, first-version workflow execution requirement

**Workflow Source**:
The editable template body and metadata fields inside a workflow definition's canonical schema.
_Avoid_: hidden prompt, generated-only workflow

**Workflow Step Graph**:
The structured step model for workflows that need visible phases, dependencies, gates, or artifacts.
_Avoid_: separate workflow system, visual-only diagram

**Workflow Schema**:
The canonical structured representation of a workflow definition, including metadata, source fields, variables, steps, gates, and artifacts.
_Avoid_: secondary Markdown truth, UI-only schema

**Workflow Schema YAML**:
The user-visible YAML representation of a workflow definition's canonical schema, used for authoring review, copy, import, export, and migration.
_Avoid_: daemon-interpreted YAML script, independent source of truth

**Workflow Schema Editor**:
The advanced workflow builder surface for editing or inspecting the complete workflow schema, typically rendered with YAML syntax but labeled as Schema in the product UI.
_Avoid_: YAML tab, Markdown Source

**Workflow Applicability**:
The trigger types a workflow revision is valid for, such as assignment, comment response, chat, or autopilot run.
_Avoid_: universal workflow by accident

**Workflow Capability**:
An agent-declared ability or constraint describing which workflow features it can execute safely.
_Avoid_: agent default workflow

**Workflow Capability Warning**:
A non-blocking warning that the selected agent may not fully support the workflow features selected for a task.
_Avoid_: hard capability gate in the first version

**Workflow Preview**:
A UI rendering of the resolved workflow content before it is saved or assigned to execution.
_Avoid_: raw-only editor, blind prompt injection

**Workflow Graph Viewport**:
The visual viewport for inspecting workflow step graph layout, including fit-to-view and zoom controls that do not change workflow schema.
_Avoid_: schema-affecting canvas state, graph editing by accident

**Workflow Builder**:
The Multica UI surface for editing workflow schemas using the product's shared UI components and design system.
_Avoid_: ai-desk UI clone, dependency-first graph canvas

**Workflow Change Review**:
The pre-publish confirmation surface that compares an active draft revision against the current published revision before an authorized human publishes the draft.
_Avoid_: workflow artifact review, preview tab

**Workflow Schema Import**:
A definition-time conversion from an external workflow document into Multica's canonical workflow schema.
_Avoid_: runtime YAML, unvalidated template paste

**Workflow Import Warning**:
A non-blocking notice that an imported workflow document included unsupported, intentionally skipped, or suspicious-but-recoverable content.
_Avoid_: silent migration loss

**Workflow Schema Export**:
A definition-time rendering of a Multica workflow schema for migration, review, or backup.
_Avoid_: second workflow source of truth

**Direct Task Workflow**:
A lightweight workflow definition for focused execution that still preserves minimum context, execution, verification, and reporting requirements.
_Avoid_: no workflow, quick skip

**Research Note Workflow**:
A lightweight workflow definition for investigation-only work that produces findings instead of code changes.
_Avoid_: research mode, ad hoc investigation

**Comment Response Workflow**:
A lightweight workflow definition for tasks triggered by mentioning an agent in an issue comment.
_Avoid_: silently continuing the full project workflow

## Relationships

- A **Workspace** owns zero or more **Workflow Definitions**.
- A **Workspace** may include **System-Seeded Workflow Definitions**.
- A **System-Seeded Workflow Definition** can be copied into a **Workflow Fork**.
- A **Workflow Fork** starts as a draft-only **Workflow Definition** until the user publishes it.
- A user-created **Workflow Definition** starts with a **Workflow Draft Revision** and no published revision.
- A **Workflow Definition** has exactly one canonical **Workflow Schema**.
- A **Workflow Definition** evolves through **Workflow Revisions**.
- A **Workflow Revision** may be draft, published, or deprecated.
- A **Workflow Draft Revision** is the required intermediate state for workflow edits before those edits can affect newly queued workflow runs.
- A **Workflow Definition** has at most one active **Workflow Draft Revision** at a time.
- A **Workflow Revision** stores its **Workflow Schema** as one versioned definition payload rather than splitting definition steps, artifacts, and gates into separate editable definition records.
- A **Workflow Revision** may expose **Workflow Schema YAML** as its reviewable definition artifact while storing the normalized **Workflow Schema** for validation, preview, snapshotting, and execution.
- A **Workflow Schema** includes **Workflow Source** and may include a **Workflow Step Graph**.
- A **Workflow Schema** declares **Workflow Applicability**.
- **Workflow Schema YAML** and the **Workflow Builder** are two editing projections over the same **Workflow Schema** and must round-trip without creating a second persisted model.
- **Workflow Schema YAML** is generated from the normalized **Workflow Schema**; user-entered YAML formatting and comments are not retained after save.
- The current Builder iteration may keep the Schema tab as canonical JSON editing while labeling it as Schema; YAML import/export is a separate migration surface.
- A **Workflow Preview** renders from the **Workflow Schema** using Multica's current UI library and design system.
- A **Workflow Graph Viewport** supports zoom in, zoom out, and fit-to-view as view-only controls.
- Workflow graph zoom controls belong to the Preview graph viewport, not to the Steps outline or **Workflow Step Inspector**.
- A **Workflow Builder** edits **Workflow Schemas** without introducing a new graph-canvas dependency in the first version.
- The workflow builder labels the full YAML-backed definition surface as **Workflow Schema Editor** or Schema rather than YAML or Markdown Source.
- A **Workflow Builder** opens the active **Workflow Draft Revision** by default when one exists, while execution and selectors continue to use the published revision.
- A **Workflow Builder** presents **Workflow Step Contracts** as the default step editing surface and keeps lower-level schema fields in advanced editing surfaces.
- A **Workflow Builder** presents workflow steps as a linear main path by default; dependency edges are inferred from order unless the user opens an advanced step-graph surface.
- A **Workflow Builder** shows workflow steps as a collapsed, scannable outline by default and edits the selected step through one **Workflow Step Inspector**.
- A workflow step outline row shows the step title as content, with inline edit affordance such as an icon button, rather than rendering a labeled Title form field.
- First-version step ordering uses explicit controls such as move up and move down rather than drag-and-drop.
- In desktop layouts, the Steps tab uses a split view with the step outline on the left and the **Workflow Step Inspector** on the right; narrow layouts may stack the inspector below the outline.
- A **Workflow Step Inspector** organizes settings by user intent: Basics, Instructions, Inputs, Outputs, and Checks.
- **Workflow Step Inspector Sections** are projections over canonical **Workflow Schema** fields and do not recreate ai-desk field-name tabs.
- Selecting a different step from the outline opens the **Workflow Step Inspector** on Basics by default.
- Step outline affordances may deep-link into a specific inspector section or the **Workflow Prompt Focus Editor** when the user explicitly clicks that affordance.
- A **Workflow Step Inspector** treats linear step order as the default input model and exposes arbitrary `depends_on` editing only in the Inputs advanced area.
- A **Workflow Prompt Focus Editor** may edit long-form prompt fields for the selected step, then apply changes back to the active **Workflow Draft Revision** autosave path.
- A **Workflow Prompt Focus Editor** returns the user to the same selected step and inspector section after apply or cancel.
- A **Workflow Builder** automatically saves edits into the active **Workflow Draft Revision**, while publishing remains an explicit user action.
- A **Workflow Draft Revision** may contain a structurally parseable but not publishable schema; publishing requires full validation.
- Invalid **Workflow Schema YAML** that cannot be parsed is not saved to the server in the first version; the builder keeps it as local unsaved editor state until the syntax is fixed.
- Workflow draft responses include backend-derived publishability and validation issues for the draft schema.
- Editing a published workflow creates the active **Workflow Draft Revision** on the first actual change, not when the builder is merely opened.
- Workflow metadata that affects understanding or execution selection, including name, description, and applicability, is edited through the active **Workflow Draft Revision** rather than mutating the published definition in place.
- A **Workflow Step Contract** uses a **Workflow Step Completion Expectation** by default; it declares explicit **Workflow Artifacts** only when an output must be reviewed, reused by later steps, or retained as a deliverable.
- The Workflow Builder may label **Workflow Step Completion Expectation** as "Done when" in the UI, but it must explain that the field describes the expected result and does not automatically complete the step.
- A **Workflow Artifact** is an explicit opt-in output in the Workflow Step Inspector Outputs section, not a required field for every step.
- The Builder distinguishes "Done when" from **Workflow Artifact**: "Done when" guides completion expectations, while artifacts create durable outputs for review, reuse, or retention.
- A **Workflow Step Contract** uses one **Workflow Step Gate** in the default editor instead of exposing review and quality-gate internals as separate required configuration groups.
- A **Workflow Step Gate** appears as one combined choice in the step outline and Basics section, while the Checks section exposes detailed human review and quality gate configuration for the selected choice.
- **Workflow Step Contracts** are projections over **Workflow Schema** fields and do not introduce UI-only persisted fields.
- Workflow change review compares the active **Workflow Draft Revision** against the current published revision; draft-only workflows show an initial publish preview.
- Workflow change review defaults to a **Workflow Step Contract** diff and provides a **Workflow Schema YAML** diff for complete definition review.
- Workflow change review can show non-publishable draft changes, but publishing is blocked by blocking validation issues.
- **Workflow Change Review** is a pre-publish action or dialog, not a persistent builder tab.
- Publishing a **Workflow Draft Revision** requires an authorized human to confirm in workflow change review, but first-version workflow authoring does not create a separate approval workflow.
- **Workflow Schema Import** and **Workflow Schema Export** are definition-time operations; runtime execution uses normalized **Workflow Schemas**, **Workflow Snapshots**, and **Workflow Runs**.
- **Workflow Schema Import** fails on structural errors that would make execution invalid and emits **Workflow Import Warnings** for intentionally skipped or recoverable fields.
- **Workflow Builder** belongs in Settings because it configures workflow definitions.
- A **Project** may have exactly one **Project Workflow Binding**.
- An **Issue** may have at most one **Issue Workflow Override**.
- **Project Workflow Bindings** and **Issue Workflow Overrides** can only select **Workflow Definitions** with a published **Workflow Revision** for the target trigger.
- Workflow selection controls only list **Workflow Definitions** with a published revision for the target trigger; draft-only workflow definitions remain visible only in workflow-authoring surfaces.
- Workflow selectors filter choices by **Workflow Applicability**.
- Workflow selectors and workflow runs display published workflow metadata, while the **Workflow Builder** displays draft metadata when an active draft exists.
- Draft-only **Workflow Definitions** may be deleted because they have no runnable published revision; published workflow definitions are archived rather than hard-deleted.
- Discarding an active **Workflow Draft Revision** is irreversible in the first version and leaves the published revision unchanged.
- Workflow definition API responses expose published and draft revisions separately; `current_published_revision_id` remains the execution pointer and must not be overloaded to mean active draft.
- An **Issue Workflow Override** takes precedence over a **Project Workflow Binding**.
- A **Project Workflow Binding** takes precedence over the workspace default workflow.
- An issue assignment **Agent Task** resolves workflow selection and stores exactly one **Workflow Snapshot** when the task is queued.
- A comment-triggered **Agent Task** defaults to the **Comment Response Workflow** instead of inheriting the project's full workflow.
- A comment-triggered **Agent Task** uses the full issue workflow only when the triggering action explicitly requests it.
- An existing **Workflow Snapshot** is not changed by later edits or publishes.
- A **Workflow Run** is created from exactly one **Workflow Snapshot** as soon as the agent task is queued.
- First-version workflow execution creates one **Workflow Run** for one agent-task queue row.
- A **Workflow Run** does not change its workflow content after creation; corrections require a new workflow revision and a newly queued run.
- A **Workflow Run** with an incorrect workflow snapshot is cancelled or rerun from a corrected workflow revision rather than repaired in place.
- A **Workflow Step Retry** preserves the current **Workflow Run** and **Workflow Snapshot**.
- A **Workflow Rerun** creates a new agent-task queue row, **Workflow Snapshot**, and **Workflow Run**.
- A **Workflow Run** contains one or more **Workflow Step Runs**.
- **Workflow Runs**, **Workflow Step Runs**, **Workflow Artifacts**, **Workflow Reviews**, and **Workflow Quality Gates** are materialized runtime records, not editable workflow definition records.
- A **Workflow Step Run** records step-level status, dependency readiness, execution metadata, and review or quality state when the workflow schema requires those features.
- **Workflow Step Run Status** uses Multica runtime language rather than ai-desk step status names.
- A **Workflow Step Execution Kind** describes step intent only; assignment, runtime, and daemon selection remain owned by Multica task and agent routing.
- A **Manual Workflow Step** can be completed by a human workspace member with optional artifact attachment or change request.
- An **External Workflow Step** can be represented as waiting or blocked in the first version, but does not trigger an external integration.
- A **Workflow Execution Batch** may execute multiple ready **Workflow Step Runs** using the same work directory or agent session.
- A **Workflow Execution Batch** stops before a **Workflow Step Run** that requires human review, a blocking quality gate, a pause, user input, or recovery from failure.
- A **Workflow Input Request** belongs to exactly one **Workflow Step Run** and is answered by an issue comment that is explicitly routed as the response.
- A **Workflow Input Request** is an agent-step waiting point, not a **Manual Workflow Step**; the human supplies input, then the agent resumes the same step.
- A **Workflow Input Request** pauses the current **Workflow Execution Batch** without creating a **Workflow Rerun**.
- Answering a **Workflow Input Request** resumes the same **Workflow Run** with a new **Workflow Execution Batch**; it does not create an ordinary comment-triggered workflow task.
- Early requirements exploration belongs in **Chat Sessions** or planning workflows before implementation issues are created; **Workflow Input Requests** inside an existing issue are for resolving the narrow input needed to continue that issue's current workflow.
- **Workflow Input Requests** are bounded: execution steps should ask at most one round by default, while planning or contract steps may allow a small configured number of rounds before reporting the remaining decision gap.
- First-version agents control workflow execution through **Workflow Execution Commands** exposed by the Multica CLI.
- First-version workflow execution respects **Workflow Step Graph** dependencies but executes ready **Workflow Step Runs** in deterministic topological order rather than parallelizing branches.
- A **Workflow Step Run** may produce zero or more **Workflow Artifacts**.
- A **Reviewable Workflow Artifact** is explicitly saved by the agent or human; Multica does not infer it by scanning local files.
- Planning, brainstorming, and requirements documents that need cloud approval are **Reviewable Workflow Artifacts**, not ordinary local outputs.
- Approving a **Reviewable Workflow Artifact** unblocks dependent workflow steps; approval does not directly create issues or perform other workspace side effects.
- Workflow steps after artifact approval may generate **Chat Issue Proposals** or create issues through explicit existing issue-creation flows.
- A **Workflow Artifact Input** describes data flow into a **Workflow Step Run** and is distinct from **Workflow Step Graph** dependency readiness.
- A **Workflow Artifact Template** is embedded in the workflow schema for first-version migration and snapshot behavior.
- A **Workflow Artifact** may store reviewable text, Markdown, JSON, or other explicitly saved content on the server.
- **Workflow Artifact Versions** are immutable; retries create new artifact versions instead of overwriting prior reviewable content.
- A **Workflow Artifact Diff** helps reviewers understand what changed between versions, while each **Workflow Artifact Version** remains a complete artifact.
- Agents may use patch-style or section-level edits to reduce generation cost, but the saved **Workflow Artifact Version** is still the complete revised document.
- Local task outputs that are not explicitly saved as **Workflow Artifacts** remain governed by **Output Metadata** privacy boundaries.
- A **Workflow Review** belongs to a **Workflow Artifact** or to the **Workflow Step Run** that produced it.
- A required **Workflow Review** is approved or rejected by a human workspace member.
- An agent may submit a **Workflow Artifact** or **Workflow Quality Gate** result, but may not approve its own required **Workflow Review** in the first version.
- A **Workflow Quality Gate** evaluates a **Workflow Artifact** using criteria captured by the workflow schema and stores its result with the **Workflow Step Run**.
- A **Workflow Quality Gate** may be blocking or non-blocking according to the workflow schema.
- A first-version **Workflow Quality Gate Result** may be produced by the executing agent and must expose that provenance.
- An agent-produced **Workflow Quality Gate Result** is not presented as a server-trusted platform verdict.
- Dependent **Workflow Step Runs** wait for required **Workflow Artifacts**, **Workflow Reviews**, and **Workflow Quality Gates** when the workflow schema declares them.
- **Party Mode** fields from ai-desk workflows are ignored during migration and are not retained as legacy workflow metadata.
- ai-desk workflow engine labels such as `NONE` and `AETHER` are not migrated as Multica workflow concepts; their supported behavior is represented through workflow schema capabilities and run semantics.
- ai-desk workflow-level automatic sidecar agents are not migrated as a Multica workflow concept; multi-agent workflow behavior must be designed through Multica-native assignment, mention, or squad semantics.
- ai-desk `AUTOMATION` workflow type is replaced by Multica **Autopilot** behavior rather than migrated as a workflow type.
- ai-desk `OFFICIAL` workflow source maps to **System-Seeded Workflow Definition**; ai-desk `PUBLIC` and `PERSONAL` imports map to user-owned **Workflow Definitions** in the target workspace.
- ai-desk workflow participants and editor collaborators are not migrated as workflow-specific ACLs; imported workflows follow Multica workspace permissions.
- ai-desk `ticket_required` is not migrated as a Multica workflow field; issue, chat, comment, and autopilot trigger fit are expressed through **Workflow Applicability** and **Autopilot**.
- ai-desk historical tasks, steps, artifacts, and workflow runs are not migrated into Multica workflow runtime tables.
- ai-desk AI-generated workflow creation is not part of first-version workflow migration.
- A daemon executes work for a **Workflow Run** and reports state back, but does not own the workflow definition or mutate the workflow snapshot.
- Browser surfaces edit **Workflow Definitions** and observe **Workflow Runs**; they do not orchestrate workflow execution state.
- **Workflow Runs** are primarily observed from the issue, task, chat, or autopilot context that created them rather than from a top-level workflow-run navigation surface.
- Completing a **Workflow Run** does not automatically post issue comments or change issue status; the workflow's explicit final reporting step remains responsible for user-visible delivery.
- An **Agent** may declare **Workflow Capability**, but does not own a default workflow.
- A mismatch between **Workflow Capability** and selected workflow features produces a **Workflow Capability Warning**.
- A **Workflow Capability Warning** is recorded with the **Workflow Snapshot** but does not block first-version execution.
- Workflow selection remains owned by issue, project, and workspace resolution.
- **Project Workflow Bindings** and **Issue Workflow Overrides** reference workflow definition IDs, not hardcoded slugs; queue-time resolution snapshots the definition's current published revision.

## Example Dialogue

> **Dev:** "This project uses the full Trellis workflow, but this bug is a two-line fix. Should the assigned agent skip workflow?"
> **Domain expert:** "No. Select the Direct Task workflow as the issue override. It is lighter, but it still records context, verification, and final reporting."

> **Dev:** "The agent asked a clarification question in the issue comments. Should the user's reply trigger the normal comment workflow?"
> **Domain expert:** "No. If the reply is submitted through the answer action, it resolves the Workflow Input Request and resumes the existing run."

> **Dev:** "The planning agent wrote a plan locally. Can reviewers approve it from the cloud?"
> **Domain expert:** "Only after the agent saves it as a Reviewable Workflow Artifact; ordinary local outputs stay behind the Output Metadata privacy boundary."

## Flagged Ambiguities

- "workflow" was used to mean both a reusable template and a running task process. Resolved: use **Workflow Definition** for the editable template and **Workflow Snapshot** for the per-task execution copy.
- "skip workflow" suggests bypassing execution governance. Resolved: use **Issue Workflow Override** to choose a lighter **Workflow Definition** instead.
- Markdown source and step graph could drift if stored independently. Resolved: **Workflow Schema** is the only source of truth; Markdown is an editable field and rendered output, not a second persisted model.
- YAML could become a daemon-interpreted runtime script, but that would couple execution recovery to a text format. Resolved: **Workflow Schema YAML** is the user-visible definition artifact; publish/import compiles it into the normalized **Workflow Schema**, and workflow runs execute from snapshots of that normalized schema.
- Workflow authoring could retain raw user-entered YAML text, but that would create drift between raw YAML, normalized schema, and the builder projection. Resolved: saved YAML is canonical generated output from the normalized **Workflow Schema**, with explanatory comments limited to templates/help views.
- The full definition editor could be labeled YAML because it uses YAML syntax, but that exposes the file format rather than the product concept. Resolved: the UI labels this surface Schema / **Workflow Schema Editor**.
- Built-in templates could remain hardcoded or become editable. Resolved: built-ins enter workspaces as read-only **System-Seeded Workflow Definitions**; user changes happen through **Workflow Forks**.
- Saving a workflow edit could silently affect task execution. Resolved: edits create draft **Workflow Revisions**; only publishing makes a revision available for new task resolution.
- Workflow edits could live only in browser state until publish, but that prevents durable review, reload recovery, and YAML inspection before publishing. Resolved: workflow edits persist as **Workflow Draft Revisions** before publish.
- Workflow definitions could allow multiple active drafts, but that would require branch naming, merge behavior, conflict handling, and per-draft selection. Resolved: each **Workflow Definition** has at most one active **Workflow Draft Revision**.
- Forking a system workflow could immediately create a published user workflow, but that would make an unreviewed copy selectable for runs. Resolved: **Workflow Forks** start draft-only and cannot be bound or queued until published.
- Creating a user workflow could immediately publish its initial revision, but that would bypass review for a runnable configuration. Resolved: user-created **Workflow Definitions** start draft-only and become selectable only after publish.
- Project and issue selectors could show draft-only workflows as disabled options, but that turns an execution selector into an authoring status surface. Resolved: workflow selection controls show published, applicable workflow definitions only.
- Workflow Builder could default to the published revision and hide draft changes behind a toggle, but that makes saved draft edits look lost. Resolved: builder opens the active draft by default and clearly labels the published revision that remains active for execution.
- Review changes could compare arbitrary historical workflow revisions, but first-version authoring only needs the pending publish decision. Resolved: Review changes compares active draft against current published, with draft-only workflows using an initial publish preview.
- Review changes could show only YAML diff, but that would make users inspect implementation-shaped fields before understanding process changes. Resolved: Review changes defaults to **Workflow Step Contract** diff and keeps **Workflow Schema YAML** diff available for full auditability.
- Review changes could hide until a draft is publishable, but users need to understand and fix invalid drafts. Resolved: Review changes can open for non-publishable drafts, while Publish is disabled for blocking validation issues.
- Review changes could be a builder tab beside Preview, but that confuses authoring preview with the human publish decision. Resolved: **Workflow Change Review** is an action/dialog shown when draft changes exist.
- Publishing workflow definition changes could create a separate approval request, but that would expand workflow authoring into another governance workflow before permissions and notifications are designed. Resolved: first-version publish approval is the authorized human confirmation inside Review changes.
- Workflow Builder could require manual save before review or publish, but that makes draft review depend on hidden unsaved browser state. Resolved: builder autosaves draft revisions and keeps publish as the explicit runnable-configuration boundary.
- Draft autosave could require full publish validation, but partial edits often pass through temporarily invalid states. Resolved: draft save accepts structurally parseable schemas with validation issues, while publish enforces full execution validation.
- Invalid YAML could be saved as raw draft source, but that would make `workflow_revision.schema` alternate between executable schema and invalid text. Resolved: unparseable **Workflow Schema YAML** remains local unsaved editor state until it can be parsed.
- Draft validation could live only in the browser, but publishability rules must match server behavior. Resolved: backend responses include derived draft validation issues and publishability.
- Opening a published workflow could eagerly create a draft, but that would produce meaningless draft changes for read-only inspection. Resolved: the first actual edit creates the active **Workflow Draft Revision**.
- Workflow metadata edits could update the definition row immediately while step edits remain in draft, but that would make selectors and reviews describe a different workflow than the one currently published. Resolved: semantic metadata edits are part of the active **Workflow Draft Revision** and affect runnable selection only after publish.
- Selectors could display draft metadata for a workflow with pending changes, but then the selected name would not match the published revision used for execution. Resolved: execution selectors and workflow runs display published metadata; builder surfaces display draft metadata with clear draft-change state.
- Draft-only workflows could be archived like published workflows, but they have no execution history or selector references to preserve. Resolved: draft-only workflow definitions may be deleted, while published workflow definitions are archived and active drafts may be discarded.
- Discarded drafts could be recoverable through a trash or version browser, but that adds another history surface before workflow authoring is stable. Resolved: discarding an active **Workflow Draft Revision** is irreversible after confirmation.
- Workflow APIs could keep a single `current_revision`, but that becomes ambiguous once the builder opens drafts by default while execution uses published revisions. Resolved: API responses expose published and draft revisions separately, with `current_published_revision_id` reserved for execution.
- Workflow definition steps, artifacts, and gates could be normalized as separate editable records, but that would complicate draft, publish, fork, import, export, and snapshot behavior. Resolved: **Workflow Schema** remains one canonical versioned definition payload, while execution state is materialized through **Workflow Runs** and related runtime records.
- Workflow Builder could copy ai-desk's ReactFlow canvas, but that would introduce a new dependency before the schema and run contract are stable. Resolved: first-version **Workflow Builder** uses Multica shared UI components, structured inspectors, drag ordering where useful, and graph preview without a new graph-canvas dependency.
- Workflow Builder could expose raw step schema fields by default, but that makes users edit implementation details before confirming the step's intent. Resolved: the default step editor uses **Workflow Step Contracts** and moves raw identifiers, dependencies, templates, artifact details, and route internals to advanced surfaces.
- Workflow Builder could expand a full editor with tabs under every step row, but that turns long workflows into repeated form stacks and makes scanning difficult. Resolved: the default Steps view is a collapsed step outline, with one **Workflow Step Inspector** for the selected step.
- Workflow Step Inspector could open as a drawer or replace the step list, but that hides the workflow outline while editing. Resolved: desktop uses a split view so users can scan the outline and edit the selected step together.
- Workflow Step Inspector sections could copy ai-desk's Basic, Prompt, Dependencies, Artifact, Rules, Party, and Quality tabs, but that exposes implementation history and preserves skipped Party Mode. Resolved: the inspector uses user-intent sections: Basics, Instructions, Inputs, Outputs, and Checks.
- Prompt editing could send users to the full Schema editor, but that makes everyday prompt changes depend on locating the right schema path. Resolved: long-form step prompts use a **Workflow Prompt Focus Editor** opened from the selected step inspector.
- Workflow Builder UX improvements could also add YAML parsing and import/export, but that expands the draft autosave and round-trip surface. Resolved: this iteration keeps Schema as canonical JSON editing and treats YAML import/export as separate work.
- Step Contract UI could store separate simplified fields and translate later, but that would create another model to keep in sync with YAML and execution. Resolved: **Workflow Step Contracts** are direct projections over **Workflow Schema** fields.
- Workflow Builder could make users author arbitrary DAG dependencies in the default step list, but first-version execution is serial and deterministic even when the schema has branches. Resolved: the default builder uses a linear main path with advanced editing for non-linear dependency graphs.
- Workflow Builder could make step ordering drag-and-drop in the first version, but drag interactions add keyboard, mobile, scroll, and dependency repair complexity. Resolved: use explicit move controls first and leave drag-and-drop out of v1.
- Workflow Step Inspector could expose every step dependency as a default field, but that turns ordinary serial workflows into graph authoring. Resolved: default Inputs represent linear order, while arbitrary `depends_on` editing lives behind an advanced control.
- Workflow Builder could require every step to name a **Workflow Artifact**, but that forces users to invent implementation outputs for ordinary progress steps. Resolved: default steps use **Workflow Step Completion Expectations** and only introduce explicit artifacts when output needs review, reuse, or durable retention.
- The "Done when" label could be interpreted as an executable completion rule, but it maps to ai-desk `outputDescription`, which is an expected result description. Resolved: keep the concise label with an inline info affordance that explains the field does not auto-complete the step.
- Workflow Builder could show required human review and quality gates as separate always-visible field groups, but that makes every step feel like a governance form. Resolved: the default editor uses one **Workflow Step Gate** with detailed reviewer, blocking, rubric, and artifact-target settings in advanced surfaces.
- Workflow run UI could live under Settings next to the builder, but running workflows belong to the work item that triggered them. Resolved: Settings owns **Workflow Builder** for definitions; issue, task, chat, or autopilot surfaces own **Workflow Run** observation.
- Workflow completion could automatically comment or close issues, but that risks duplicate or context-free reporting. Resolved: final user-visible reporting remains an explicit workflow step executed by the agent or human, while the server only derives **Workflow Run** state.
- YAML import and export could be treated as a peripheral migration tool, but ai-desk workflow authoring uses YAML as the reviewable definition artifact. Resolved: first-version workflow migration includes **Workflow Schema YAML** for review, copy, import, and export, while runtime execution uses normalized schema snapshots rather than direct YAML interpretation.
- Workflow import could fail on every unsupported ai-desk field or silently drop them, but both choices are poor for migration. Resolved: structural errors fail import; intentionally skipped or recoverable fields produce **Workflow Import Warnings** and continue.
- ai-desk supports creating workflows with AI-generated YAML, but that would add unstable schema generation and repair to the migration surface. Resolved: first-version migration excludes AI-generated workflow creation; future Multica workflow proposal flows can be designed separately.
- Workflow artifact content could remain daemon-local, but review, quality gates, and dependent steps need stable cross-device inputs. Resolved: explicit **Workflow Artifacts** are server-stored reviewable content, while ordinary local outputs remain **Output Metadata** unless explicitly saved as artifacts.
- Planning documents could be treated as local outputs with metadata only, but that prevents cloud review and approval. Resolved: brainstorming, requirements, and planning documents that require approval are saved explicitly as **Reviewable Workflow Artifacts**.
- Plan approval could automatically create issues, but that would conflate artifact review with workspace side effects. Resolved: approving a **Reviewable Workflow Artifact** only unblocks the workflow; a later explicit step creates **Chat Issue Proposals** or issues.
- Step retry could overwrite the existing artifact, but that would erase review and quality history. Resolved: retries create new **Workflow Artifact Versions**, with UI defaulting to the latest version while preserving history.
- Artifact revisions could be stored as patches only, but that would make review and downstream execution depend on replaying prior versions. Resolved: each **Workflow Artifact Version** is stored as a complete artifact; **Workflow Artifact Diffs** are generated or stored as review aids.
- Workflow reviews could allow agent self-approval, but that would collapse human review into automated quality evaluation. Resolved: first-version required **Workflow Reviews** are approved or rejected by human workspace members.
- Quality gates could always block or never block later steps. Resolved: **Workflow Quality Gates** declare blocking behavior in the workflow schema; blocking gates hold dependent **Workflow Step Runs**, non-blocking gates record reports and warnings.
- Quality gates could require a server-side evaluator in the first version, but ai-desk gates are often prompt/rubric driven and expensive to rehost immediately. Resolved: first-version **Workflow Quality Gate Results** may be agent-produced with explicit provenance, not anonymous platform verdicts.
- Step-level execution state could imply a cold-started daemon task for every step, increasing token and runtime overhead. Resolved: **Workflow Step Runs** remain independent state units, while a **Workflow Execution Batch** may reuse session and work directory across consecutive ready steps until a blocking boundary is reached.
- Workflow step control could be exposed first through MCP tools, but Multica already has a daemon-to-agent CLI contract. Resolved: first-version workflow execution uses **Workflow Execution Commands** in the Multica CLI; MCP can be added later as an ergonomic layer.
- Human clarification could be modeled as another issue status or an ordinary comment mention, but that would either overload the issue board or accidentally trigger comment response workflows. Resolved: use **Workflow Input Requests** for explicit human answers and treat their replies as workflow-resume inputs, not ordinary comment-trigger tasks.
- Resuming after human input could create a new workflow run, but that would fragment step history and make the original wait state look terminal. Resolved: answering a **Workflow Input Request** resumes the same **Workflow Run** with another **Workflow Execution Batch**.
- Open-ended brainstorming could happen inside an implementation issue, but that turns issue execution into requirements discovery and makes work-in-progress misleading. Resolved: use **Chat Sessions** or planning workflows for initial requirements exploration; use **Workflow Input Requests** only for bounded clarification needed to continue an existing issue workflow.
- Human input could be represented by **Manual Workflow Steps**, but that would imply the human completes the step rather than merely answering the agent. Resolved: **Workflow Input Requests** are agent-step waiting points; **Manual Workflow Steps** remain human-completed steps.
- Workflow input could allow unlimited question loops, but that would hide unready work inside active issues. Resolved: **Workflow Input Requests** are round-limited by workflow policy; after the limit, the workflow reports the unresolved decision gap instead of continuing to ask.
- A workflow DAG could execute ready branches in parallel, but that would introduce artifact merge, session concurrency, workdir write conflict, review ordering, and token attribution complexity. Resolved: first-version **Workflow Step Graph** execution is serial and deterministic even when the graph contains independent ready branches.
- Step dependencies and artifact inputs could be merged during import, but that would conflate control flow with data flow. Resolved: `depends_on_steps` controls **Workflow Step Run** readiness; **Workflow Artifact Inputs** declare readable or required artifact data, with import warnings for suspicious mismatches rather than automatic graph rewrites.
- ai-desk artifact templates could be migrated as a reusable catalog first, but that would add another dependency before workflow definitions can be self-contained. Resolved: first-version migration embeds **Workflow Artifact Templates** in the workflow schema; unresolved ai-desk template references produce import warnings.
- ai-desk step statuses could be copied directly, but their names encode ai-desk UI and service history. Resolved: **Workflow Step Run Status** uses Multica-oriented states for readiness, execution, review, quality, manual, external, pause, block, failure, completion, and skip.
- Workflow resolution could happen when an agent claims work, but that would let delayed claims observe newer publishes than the user assigned. Resolved: issue assignment tasks snapshot workflow at queue time.
- Workflow runs could be created lazily when a daemon claims work, but that would delay visibility and risk observing workflow edits made after assignment. Resolved: **Workflow Snapshot**, **Workflow Run**, and initial **Workflow Step Runs** are created at agent-task queue time.
- A workflow run could span multiple agent-task queue rows, but that would complicate existing task lifecycle, usage, cancellation, and runtime recovery semantics. Resolved: first-version **Workflow Run** has a 1:1 relationship with the queue row that created it.
- Workflow content could be patched during a running workflow, but that would make execution history ambiguous. Resolved: a **Workflow Run** is immutable with respect to workflow content after creation.
- Current-run workflow repair could reduce friction, but it breaks snapshot immutability and auditability. Resolved: incorrect workflow content is fixed by editing or publishing a new **Workflow Revision** and starting a new **Workflow Run**.
- Retry could mean either redoing one failed step or starting over with a new workflow snapshot. Resolved: **Workflow Step Retry** re-executes a retryable step inside the same run; **Workflow Rerun** creates a new queue row and workflow run.
- Workflow execution ownership could live in the browser, daemon, or server. Resolved: **Workflow Runs** are server-owned; daemons execute and report, while browser surfaces edit definitions and observe runs.
- Workflow steps could be rendered only as prompt sections, but that would lose dependency, artifact, review, pause, and quality behavior. Resolved: step execution uses **Workflow Step Runs**, not prompt-only rendering.
- Step execution could omit artifact, review, and quality concepts at first, but that would make the ai-desk workflow model impossible to preserve without later schema churn. Resolved: first-version **Workflow Step Runs** include **Workflow Artifacts**, **Workflow Reviews**, and **Workflow Quality Gates** as first-class run semantics.
- Party Mode could be migrated with ai-desk workflows, but it would force an early decision between simulated roles and Multica's real multi-agent model. Resolved: **Party Mode** is intentionally not migrated; future Multica multi-agent workflow design will define its own model.
- ai-desk `WorkflowEngine` labels could be preserved, but they describe ai-desk execution paths rather than Multica domain concepts. Resolved: Multica does not migrate `NONE` or `AETHER`; it models supported behavior through schema capabilities and **Workflow Run** semantics.
- ai-desk workflow templates can pre-attach sidecar agents to tasks, but that conflicts with Multica's assignment, mention, and squad model. Resolved: these sidecar-agent fields are not migrated; future multi-agent workflows must be designed using Multica-native collaboration semantics.
- ai-desk `WORKFLOW` and `AUTOMATION` could be preserved as workflow types, but Multica separates execution templates from autonomous triggering. Resolved: ai-desk `AUTOMATION` maps to **Autopilot** behavior, not to a Multica workflow type.
- ai-desk workflow source and participant models could be migrated directly, but that would introduce a second workflow ACL model. Resolved: `OFFICIAL` becomes **System-Seeded Workflow Definition**, `PUBLIC` and `PERSONAL` become user-owned **Workflow Definitions**, and ai-desk workflow participants are not migrated.
- ai-desk `ticket_required` could be carried over, but Multica uses trigger applicability rather than ticket presence as the workflow boundary. Resolved: `ticket_required` is not migrated; equivalent behavior is represented by **Workflow Applicability** or **Autopilot**.
- ai-desk historical workflow task data could be migrated, but it would carry incompatible issue, agent, runtime, permission, and state-machine assumptions into new Multica runtime tables. Resolved: migration imports workflow definitions/templates only; ai-desk historical runs remain outside Multica workflow runtime state.
- ai-desk step `ExecutionMode` could be copied as `MANUAL`, `BUILT_IN_AGENT`, `LOCAL_AGENT`, and `EXTERNAL_AGENT`, but those names mix step intent with ai-desk runtime routing. Resolved: Multica uses **Workflow Step Execution Kind** values such as agent, manual, and external; `agent` runs through the current Multica assignment/runtime path.
- Manual and external steps could both require full first-version integrations, but that would over-expand the migration. Resolved: **Manual Workflow Steps** get a minimal human completion loop; **External Workflow Steps** are representable as waiting or blocked without external system execution.
- Comment mentions could inherit the full project workflow, but that would make lightweight collaboration unexpectedly heavy. Resolved: comment-triggered tasks default to **Comment Response Workflow** and require explicit opt-in for full workflow execution.
- Workflows could accidentally be bound to incompatible triggers. Resolved: **Workflow Applicability** is declared in schema and used by UI/API selection rules.
- Agent-level workflow defaults would reintroduce assignment-dependent workflow changes. Resolved: agents declare **Workflow Capability** only; they do not select the workflow by default.
- Capability mismatch could block assignment, but early capability modeling will be incomplete. Resolved: first version emits **Workflow Capability Warning** and records it with the snapshot, without blocking execution.
