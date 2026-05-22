# Expose RingCentral capabilities through CLI first

RingCentral connector capabilities should be exposed to agents through Multica-controlled CLI commands first, with MCP wrappers treated as optional later adapters over the same backend capability model. The CLI surface can read the task's Project Resources, enforce the Connector Delegated User, and work across every supported runtime rather than only runtimes that consume MCP configuration.

The rejected alternative was to make RingCentral access MCP-only. Multica currently has uneven MCP support across agent providers, so an MCP-only integration would make RingCentral behavior depend on the selected runtime instead of the task's connector authorization model.
