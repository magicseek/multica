# Model chat planning as stateful plan runs with lead-led squad consensus

Multica will add chat planning as a stateful **Chat Plan Run** inside an existing **Chat Session**, not as a separate chat-session type and not as a stateless per-message toggle. A plan run uses a server-owned **Plan Engine** preset, stores lightweight planning state and a **Plan Summary**, and produces **Chat Issue Proposals** that still require explicit user approval before issues are created.

For squad-backed planning, the selected squad's lead agent remains the single coordinator. The lead agent may consult squad members through bounded **Chat Plan Consultations** in the chat transcript, but the final **Squad Plan Consensus** is the lead agent's synthesis rather than a multi-agent roundtable or unanimous vote.

## Considered Options

- Make plan mode a dedicated chat-session type. Rejected because current chat sessions are private, creator-owned conversations bound to one agent, and users need to move between ordinary chat and planning inside the same transcript.
- Treat each plan message as independent. Rejected because grill-style planning requires multiple rounds without asking the user to reselect plan mode before every reply.
- Require workspace skills or local runtime skills for planning engines. Rejected because plan behavior must be a product-level contract; local skill projection is an execution enhancement, not the source of truth.
- Run squad planning as an open roundtable. Rejected because Multica squads already use lead-agent coordination, and unbounded agent-to-agent debate can leave planning runs stuck.
