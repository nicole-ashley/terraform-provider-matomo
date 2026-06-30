# .agents

Vendored agent skills, kept here as plain files (rather than as Claude Code
plugin/marketplace references) because this environment's cloud sessions
can't reach arbitrary GitHub owners to clone marketplaces at runtime.
`.claude/skills` is a symlink to `skills/` in this directory so Claude Code
picks them up; other agent tooling that looks for `.agents/skills` will find
the same files directly.

## Sources

| Skills | Source | License |
|---|---|---|
| `brainstorming`, `dispatching-parallel-agents`, `executing-plans`, `finishing-a-development-branch`, `receiving-code-review`, `requesting-code-review`, `subagent-driven-development`, `systematic-debugging`, `test-driven-development`, `using-git-worktrees`, `using-superpowers`, `verification-before-completion`, `writing-plans`, `writing-skills` | [obra/superpowers](https://github.com/obra/superpowers) | MIT — see `LICENSE-superpowers` |
| `new-terraform-provider`, `provider-actions`, `provider-docs`, `provider-resources`, `provider-test-patterns`, `run-acceptance-tests`, `azure-verified-modules`, `terraform-search-import`, `terraform-style-guide`, `terraform-test`, `refactor-module`, `terraform-stacks`, `aws-ami-builder`, `azure-image-builder`, `windows-builder`, `push-to-registry` | [hashicorp/agent-skills](https://github.com/hashicorp/agent-skills) | MPL-2.0 — see `LICENSE-hashicorp-agent-skills` |

Vendored as a snapshot of each repo's default branch; not auto-updated. To
refresh, re-pull the `skills/` subtree from the source repo and re-copy.
