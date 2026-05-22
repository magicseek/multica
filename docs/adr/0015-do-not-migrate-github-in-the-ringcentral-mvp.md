# Do not migrate GitHub in the RingCentral MVP

The RingCentral connector MVP should not migrate the existing GitHub integration into the new connector framework. GitHub remains on its current GitHub App installation, setup callback, webhook, pull request mirror, issue-PR link, and settings-card implementation while RingCentral providers are introduced in parallel.

The connector framework may reserve or later add a GitHub provider identity, but that is outside this MVP's implementation scope.

The rejected alternative was to convert GitHub at the same time as adding RingCentral. GitHub's App installation and webhook model is materially different from RingCentral's user-owned PAT connector model, and migrating it now would expand risk without improving the RingCentral profile boundary.
