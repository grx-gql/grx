# Shared Git hooks

This directory is wired via **`git config core.hooksPath .githooks`** (see **`make install-hooks`** or **`make dev-setup`** in the repo root `Makefile`).

| Hook           | Purpose |
|--------|---------|
| **`commit-msg`** | Require a [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) **subject line** (`feat:`, `fix:`, …) so merges to `main` work well with Release Please / changelog automation. |

**Per-clone**: run **`make dev-setup`** (`go mod tidy` in all modules **then** hooks) or **`make install-hooks`** for hooks only — Git does not use this folder until `core.hooksPath` is set.

**Opt out for one commit**:

```bash
git commit --no-verify ...
```
