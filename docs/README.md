# grx documentation site

This directory is the source for the [grx documentation site](https://patrickkabwe.github.io/grx/).
It is built with [VitePress](https://vitepress.dev/).

## Local development

Commands are exposed through the repo root `Makefile` so you do not need to
remember tool-specific flags.

```bash
make docs-install   # bun install in docs/
make docs-dev       # dev server with HMR at http://localhost:4321/grx/
make docs-build     # production build → docs/.vitepress/dist
make docs-preview   # serve the built site locally
make docs-clean     # remove .vitepress/dist, .vitepress/cache, node_modules/
```

You can also run the Bun scripts directly from this directory:

```bash
bun install
bun run dev
bun run build
bun run preview
```

## Layout

```
docs/
├── .vitepress/
│   ├── config.ts           # site + sidebar + theme options
│   └── theme/
│       ├── index.ts        # extends the default theme
│       └── custom.css      # accent tweaks
├── package.json            # bun deps + scripts
├── public/                 # static assets (favicon, hero image, …)
├── index.md                # home page (layout: home)
├── getting-started.md
├── concepts/               # architecture, schema, executor, …
├── guides/                 # task-oriented walkthroughs
├── reference/              # API reference (hand-maintained + mirrored)
├── benchmarks.md
├── changelog.md            # generated — see below
├── roadmap.md              # generated — see below
└── README.md               # this file
```

## Auto-generated content

Two pages are mirrored from canonical sources elsewhere in the repo.
**Do not edit them directly** — your edits will be overwritten on the next
`make docs-content`.

| Page             | Generated from        | Script                      |
| ---------------- | --------------------- | --------------------------- |
| `changelog.md`   | repo-root `CHANGELOG.md` | `scripts/sync-changelog.sh` |
| `roadmap.md`     | repo-root `ROADMAP.md`   | `scripts/sync-roadmap.sh`   |

Regenerate both in one shot:

```bash
make docs-content
```

`docs-dev` and `docs-build` already depend on `docs-content`, so the mirrored
pages stay fresh when you serve or build the site.

## Deployment

`.github/workflows/docs.yml` builds and publishes the site to GitHub Pages on
every push to `main`. Pull requests run the same pipeline up to (but not
including) the deploy so broken builds are caught at review time.

### Pipeline

1. Checks out the repo with full history (VitePress `lastUpdated` reads git).
2. Sets up Node and Bun (pinned in the workflow).
3. `bun install --frozen-lockfile` in `docs/` (uses `docs/bun.lock`).
4. `make docs-content` regenerates the changelog and roadmap.
5. `bun run build` → `docs/.vitepress/dist`.
6. Verifies `docs/.vitepress/dist/index.html` exists before uploading.
7. Uploads `docs/.vitepress/dist` as a Pages artifact (skipped on PRs).
8. The deploy job uses `actions/deploy-pages@v4` with the `github-pages`
   environment (skipped on PRs).

### One-time GitHub setup

The workflow cannot enable Pages on its own. Do this once in the repository
settings:

1. Open **Settings → Pages** for `patrickkabwe/grx`.
2. Under **Build and deployment**, set **Source** to **GitHub Actions**.
3. (Optional) Add a custom domain via **Pages** settings and DNS.

Once Pages is enabled, every push to `main` that touches a doc-relevant path
will publish to **https://patrickkabwe.github.io/grx/**. Manual runs are also
available via **Actions → Docs → Run workflow**.

### Path filter

The workflow runs when one of these paths changes:

- `docs/**`
- `scripts/sync-changelog.sh`, `scripts/sync-roadmap.sh`, `scripts/gen-api-docs.sh`
- `CHANGELOG.md`, `README.md`, `ROADMAP.md`
- `Makefile`
- `.github/workflows/docs.yml`
- `**/*.go` (so doc-comment changes are picked up for future API reference tooling)

Use **Run workflow** for manual deploys outside that filter.

## Frontmatter cleanup

If you paste Markdown that still uses old Starlight-style frontmatter, run:

```bash
python3 scripts/normalize-vitepress-frontmatter.py
```

That script keeps `title`, `description`, heading outline levels, and
`lastUpdated: false` for generated API pages.
