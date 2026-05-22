# Use container and pinned project resources

RingCentral project resources should use two granularities. Container Project Resources bound discovery and operations inside an external container: `ringcentral_gitlab_repo` for a GitLab repository, `ringcentral_jira_project` for a Jira project, and `ringcentral_wiki_space` for a Wiki space. Pinned Project Resources attach exact external context: `ringcentral_jira_issue` for a Jira issue and `ringcentral_wiki_page` for a Wiki page.

GitLab write operations are limited to the attached `ringcentral_gitlab_repo`. Jira and Wiki search/read operations default to attached container resources, while exact issue and page context should be pinned when a task depends on a specific external object.

The rejected alternative was to attach broad provider-level resources such as "RingCentral Jira" or "RingCentral Wiki". Provider-level scope would make agent search too broad, increase accidental data exposure, and make it harder to audit why a task was allowed to read or act on a specific external object.
