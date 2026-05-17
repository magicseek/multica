# Repositories are workspace code working targets

Multica will reuse **Repository** as the product concept for code working targets rather than introducing a separate Codebase entity. Repositories are first-class workspace resources, managed from Workspace Settings and referenced by projects, chats, issues, and tasks. A repository may start as remote Git, a user-selected local directory, or a lead-agent-created managed directory; machine paths live in private repository bindings, not shared project settings.

## Considered Options

- Keep repositories as `workspace.repos` JSON plus `project_resource(github_repo)`. Rejected because local directories, agent-managed starts, per-machine bindings, publish state, and runtime eligibility need stable identities and state.
- Add a new Codebase concept. Rejected because Multica already exposes Repositories for this domain, and Project already means a planning container.

## Consequences

- Project-level repository settings store repository references, not copied URLs or paths.
- Local path visibility is controlled at the repository binding layer.
- Existing `workspace.repos` and `project_resource(github_repo)` need a migration path into first-class repositories.
