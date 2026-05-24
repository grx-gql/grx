---
title: AI assistants
description: Use Cursor Skills, AGENT.md, and AGENTS.md so coding agents behave well with grx as a dependency or inside this repo.
outline: deep
---

# AI assistants

**grx** ships tooling so AI coding agents - and humans wiring them - share the same ground rules.

## In this repository (contributors)

Read **[`AGENT.md`](https://github.com/grx-gql/grx/blob/main/AGENT.md)** in full before refactoring **`exec/`**, **`server/`**, transports, or the public API surface. It documents package boundaries, **`make test`**, race testing, resolver/schema patterns, and mistakes agents commonly make here.

Several agent products also look for **[`AGENTS.md`](https://github.com/grx-gql/grx/blob/main/AGENTS.md)** at the repo root - it distinguishes **changing `grx`** from **using `grx` in another module**.

## In your Go project (consumers)

### Cursor Agent Skills

1. Copy **`graphql-grx`** from this repository tree:

   [**`.cursor/skills/graphql-grx/`](https://github.com/grx-gql/grx/tree/main/.cursor/skills/graphql-grx)**

2. Install it where Cursor loads skills - for example (**macOS/Linux**):

   ```bash
   mkdir -p ~/.cursor/skills
   cp -r /path/to/grx/.cursor/skills/graphql-grx ~/.cursor/skills/graphql-grx
   ```

   …or symlink the folder so **`grx` pulls** upgrade the skill in place.

3. When starting a **`grx`** task in Cursor, **@**-mention **`graphql-grx`** (the skill **`name`** in the SKILL frontmatter) so transports, **`schema.Config`**, subscriptions, auth, and hardening guides stay in context.

### Other agents

Paste excerpts from **`AGENT.md`** / **`.cursor/skills/graphql-grx/SKILL.md`** into your product’s “custom instructions” - or fetch the skill from:

**`https://raw.githubusercontent.com/grx-gql/grx/main/.cursor/skills/graphql-grx/SKILL.md`**

## Human-first paths

Prefer narrative docs when onboarding people: **[Get started](/getting-started/)**, **[Define your schema](/concepts/schema-basics)**, and the grouped Guides sidebar (**Query and mutation**, **Subscriptions**, …).
