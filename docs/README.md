# grx documentation site

This directory is the source for the [grx documentation site](https://patrickkabwe.github.io/grx/).
It is built with [Astro Starlight](https://starlight.astro.build/).

## Local development

All commands are exposed through the repo root `Makefile` so you don't
need to remember bun/astro flags.

```bash
make docs-install   # bun install in docs/
make docs-dev       # dev server with HMR at http://localhost:4321/grx
make docs-build     # production build → docs/dist
make docs-preview   # serve the built site locally
make docs-clean     # nuke dist/, .astro/, node_modules/
```

You can also run the bun scripts directly from this directory:

```bash
bun install
bun run dev
bun run build
bun run preview
```

## Layout

```
docs/
├── astro.config.mjs            # Starlight + sidebar config
├── package.json                # bun deps + scripts
├── public/                     # static assets served at /
├── src/
│   ├── assets/logo.svg         # in-page logo (currentColor aware)
│   ├── content.config.ts       # Astro content collection schema
│   ├── content/docs/
│   │   ├── index.mdx           # splash home page (CardGrid)
│   │   ├── getting-started.md
│   │   ├── concepts/           # architecture, schema, executor, ...
│   │   ├── guides/             # task-oriented walkthroughs
│   │   └── reference/          # API reference (auto-generated)
│   └── styles/custom.css       # accent color + minor type tweaks
└── README.md                   # this file
```

## Auto-generated content

Two pages on the site are mirrored from canonical sources elsewhere in
the repo. **Do not edit them directly** — your edits will be overwritten
on the next build.

| Page                          | Generated from                                 | Script                          |
| ----------------------------- | ---------------------------------------------- | ------------------------------- |
| `reference/<pkg>/index.md`    | Go doc comments                                | `scripts/gen-api-docs.sh`       |
| `changelog.md`                | repo-root `CHANGELOG.md`                       | `scripts/sync-changelog.sh`     |
| `roadmap.md`                  | repo-root `README.md` (Feature Parity Checklist) | `scripts/sync-roadmap.sh`     |

Regenerate everything in one shot:

```bash
make docs-content   # docs-api + docs-changelog
```

`docs-dev` and `docs-build` already depend on `docs-content`, so the
generated pages are always fresh when you serve or build the site.

The API-ref script preserves the hand-written `reference/index.md`
landing page; it only blows away the per-package subdirectories before
regenerating.

## Deployment

`.github/workflows/docs.yml` builds and publishes the site to GitHub
Pages on every push to `main`. PRs run the same pipeline up to (but not
including) the deploy so broken builds get caught at review time.

### Pipeline

1. Checks out the repo with full history (Starlight's `lastUpdated:
   true` reads commit timestamps).
2. Sets up Go (version pulled from `go.mod`) and Bun (pinned).
3. Caches `~/go/pkg/mod` and `~/.cache/go-build` so `gomarkdoc`'s
   transitive deps are downloaded once.
4. `bun install --frozen-lockfile` in `docs/` (uses `docs/bun.lock`).
5. `make docs-content` regenerates the API reference, changelog, and
   roadmap from their canonical sources.
6. `bun run build` → `docs/dist`.
7. Verifies `dist/index.html`, `dist/_astro/`, and `dist/pagefind/`
   exist before doing anything destructive.
8. Uploads `docs/dist` as a Pages artifact (skipped on PRs).
9. The deploy job uses `actions/deploy-pages@v4` with the `github-pages`
   environment to publish (skipped on PRs).

### One-time GitHub setup

The workflow cannot enable Pages on its own. Do this once in the
repository settings:

1. Open **Settings → Pages** for `patrickkabwe/grx`.
2. Under **Build and deployment**, set **Source** to **GitHub Actions**.
3. (Optional) Add a custom domain by creating a `CNAME` file under
   `docs/public/` and configuring DNS. Astro will copy it into `dist/`
   verbatim.

Once Pages is enabled, every push to `main` that touches a doc-relevant
path will publish to **https://patrickkabwe.github.io/grx/**. Manual
runs are also available via **Actions → Docs → Run workflow**.

### Path filter

The workflow only runs when one of these paths changes:

- `docs/**`
- `scripts/gen-api-docs.sh`, `scripts/sync-changelog.sh`,
  `scripts/sync-roadmap.sh`
- `CHANGELOG.md`, `README.md`
- `Makefile`
- `.github/workflows/docs.yml`
- `**/*.go` (so doc-comment changes flow through to the API reference)

Use **Run workflow** for manual deploys outside that filter (for
example, to refresh `lastUpdated` timestamps after a string of unrelated
commits).
