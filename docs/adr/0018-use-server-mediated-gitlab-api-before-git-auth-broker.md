# Use server-mediated GitLab API before git auth broker

RingCentral GitLab connector capabilities should be API-first in the MVP. The connector adapter may read repository trees and files, create semantic task branches, create commits with file actions, push those changes through the GitLab commit API, and create merge requests. It must not inject RingCentral PATs into local git remotes, git credential helpers, agent environment variables, or task context.

Tasks that need a full local checkout, local tests, or broad code editing continue to use Multica's existing Repository and Repository Binding flows. Those flows rely on user-machine git credentials or existing local bindings, not RingCentral connector credentials.

A future Git Auth Broker may let a daemon authenticate task-scoped local git clone/push operations against connector-backed repositories without exposing raw PATs to agents.

The rejected alternative was to make RingCentral GitLab PATs available to local git in the first version. That would expand the credential exposure surface, couple connector credentials to daemon-specific git behavior, and complicate least-privilege auditing before the server-mediated connector model is proven.
