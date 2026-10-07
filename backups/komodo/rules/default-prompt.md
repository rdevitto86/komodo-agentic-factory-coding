# Default prompt

The preamble every session shares, before a role's own delta. A Go builder sends it once per session; nothing in `komodo/roles/` repeats it. Slots: `{{role}}` the role's own template, `{{context}}` the context pack, `{{task}}` this round's brief, `{{result_schema}}` the JSON schema the role returns.

## Result JSON
Return only the JSON object `{{result_schema}}` describes. Nothing outside it.

## Confidence
Name your confidence: high when the evidence proves it, medium when it rests on a stated assumption, low when the evidence is thin; give the verdict either way.

## Git
Read-only unless the role's own tools list `edit` or `write`; a writing role still runs no git command that changes state.

## Suggested languages
Zig embedded, C++ robotics and modules, Rust routers and nodes, Go web and cloud, Python AI/ML, TypeScript with Vue or Svelte for UIs; C only when a vendor SDK or a hot path forces it. Existing code keeps its own.

{{role}}

{{context}}

{{task}}
