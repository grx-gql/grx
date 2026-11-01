# grx documentation site

This directory is the source for the [grx documentation site](https://grx-gql.github.io/grx/).
It is built with [Fumadocs](https://fumadocs.dev/) on Next.js and exported as
static HTML for GitHub Pages.

## Local development

Commands are exposed through the repo root `Makefile`:

```bash
make docs-install    # bun install in docs/
make docs-dev        # dev server with hot reload at http://localhost:4321/grx/
make docs-build      # static export → docs/out
make docs-linkcheck  # verify internal links and #anchors in docs/out
make docs-preview    # serve docs/out at http://localhost:4321/grx/
make docs-clean      # remove out/, .next/, .source/, node_modules/
```

Or run the Bun scripts from this directory: `bun install`, `bun run dev`,
`bun run build`, `bun run preview`.

## Layout

```
docs/
├── app/
│   ├── (home)/page.tsx       # landing page
│   ├── (docs)/[...slug]/     # every docs page, mounted at the site root
│   ├── api/search/           # static search index (Orama)
│   ├── og/                   # generated Open Graph images
│   ├── llms.txt, llms-full.txt, llms.mdx/  # Markdown for LLMs / "Copy Markdown"
│   └── sitemap.ts
├── components/
│   ├── home/                 # landing page widgets
│   ├── mdx.tsx               # MDX component map
│   └── search.tsx
├── content/docs/             # all pages (MDX); URLs mirror this tree
│   ├── meta.json             # sidebar order and section separators
│   ├── getting-started/  concepts/  guides/  examples/  reference/
│   └── benchmarks.mdx
├── lib/
│   ├── source.ts             # Fumadocs content source
│   ├── layout.shared.tsx     # nav, logo, links
│   └── shared.ts             # site URL, base path, GitHub info
├── public/                   # static assets copied as-is
└── next.config.mjs           # basePath /grx, static export
```

## Writing pages

Pages are MDX with `title` and `description` frontmatter. Fumadocs renders the
title, so do not repeat it as a `#` heading.

- **Sidebar:** `content/docs/meta.json` lists pages in order. `---Label---`
  entries are section separators; pages from any folder can be listed, so the
  sidebar grouping is independent of URLs. Add a new page there or it will not
  appear in the sidebar.
- **Callouts:** `<Callout type="info" title="Optional title">…</Callout>`
  (`type` is `info`, `warn`, or `error`).
- **Embedding repo files:** `<include cwd>../examples/subscriptions/main.go</include>`
  renders the file as a code block (paths are relative to `docs/`).
- **Custom heading ids:** `## Heading [#custom-id]`.
- **Links:** use absolute paths without the base path, e.g. `/guides/auth`.
- **API reference** pages under `reference/` are plain Markdown generated
  by gomarkdoc; anchors use `[#Symbol]` heading ids.

## Deployment

`.github/workflows/docs.yml` builds the site on pull requests and deploys it to
GitHub Pages on every push to `main`:

1. Checks out the repo with full history (page "last updated" dates come from git).
2. Sets up Node and Bun.
3. `bun install --frozen-lockfile` in `docs/`.
4. `make docs-content` runs the derived-content step.
5. `bun run build` → `docs/out`.
6. Verifies the export and runs `make docs-linkcheck`.
7. Uploads `docs/out` as a Pages artifact and deploys it (skipped on PRs).

The site is served from `/grx/`, configured by `basePath` in
`next.config.mjs` and `basePath` in `lib/shared.ts`; keep the two in sync.
`trailingSlash: true` exports `page/index.html`, so both `/page` and `/page/`
resolve on GitHub Pages and old URLs keep working.

### One-time GitHub setup

In **Settings → Pages** for `grx-gql/grx`, set **Source** to **GitHub Actions**.
