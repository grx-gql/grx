# Releasing grx

## Automatic releases (root module)

[`release-please`](https://github.com/googleapis/release-please) bumps the semver from [**Conventional Commits**](https://www.conventionalcommits.org/) merged to **`main`** (`feat:` / `fix:` / `perf:` → minor or patch depending on semver rules; `BREAKING CHANGE` footer or **`!`** → major).

**What happens**

1. **release‑please** runs from **`.github/workflows/release.yml`** when **`main`** is updated **or** when you use **Actions → Release → Run workflow** with branch **`main`** (optional **workflow_dispatch** — same job as a push, no extra inputs). It opens or updates **one Release PR**: bumps **`.release-please-manifest.json`**, rolls commits under **`CHANGELOG.md`** headings that match **`release-please-config.json`** (emoji‑prefixed sections such as **`✨ Added`**, **`🐛 Fixed`**, **`💥 Breaking Changes`**, …).
   - If you tweak the unpublished heading in **`CHANGELOG.md`** manually, edit that file in the repo only — the **docs site does not mirror** the changelog; readers use **[`CHANGELOG.md` on GitHub](https://github.com/grx-gql/grx/blob/main/CHANGELOG.md)** (and **[Releases](https://github.com/grx-gql/grx/releases)**).
2. **Merge that Release PR** → release‑please creates the **`vMAJOR.MINOR.PATCH`** tag and **GitHub Release** for the **root** Go module (`github.com/grx-gql/grx`).
3. **Tag push** runs the same **`Release`** workflow: **`proxy.golang.org`** warmup for whatever tag landed (`v*` root or **`redis-pubsub/v*`** nested). Tests remain on **`main`** Pull Requests via **`.github/workflows/ci.yml`**.
4. **Documentation site builds** call **`make docs-content`** (mirrors **`ROADMAP.md`** → **`docs/roadmap.md`**) before **`bun run build`**.

Configuration lives in **`release-please-config.json`** and **`.release-please-manifest.json`** (typically bootstrapped from the latest **`v*`** tag).

Versioning and tags for the **root** module come only from **release‑please** (Release PR merge creates **`v*`**); the workflow offers **manual dispatch only to re-run release‑please** on **`main`**, not ad-hoc tagging. The changelog is **[`CHANGELOG.md` on GitHub](https://github.com/grx-gql/grx/blob/main/CHANGELOG.md)** — it is **not** published as a docs site page anymore.

### Permissions (fix “GitHub Actions is not permitted to create … pull requests”)

The workflow already requests **`pull-requests: write`** for the **`release-please`** job. GitHub **still rejects** [`POST /repos/.../pulls`](https://docs.github.com/en/rest/pulls/pulls?apiVersion=2022-11-28#create-a-pull-request) if the repository or organisation **never allows** **`GITHUB_TOKEN`** to open PRs, or caps workflow token scopes.

**Prefer fixing `GITHUB_TOKEN` (same repo)**

1. **Repository** (**or parent Organisation**) → **Settings** → **Actions** → **General**.
2. Under **Workflow permissions**, choose **Read and write permissions** (not **Read repository contents and packages permissions** only).
3. Enable **Allow GitHub Actions to create and approve pull requests** ([**Managing Actions settings**](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-your-repository); search the page for *create and approve pull requests* if the TOC moves).

If steps 2–3 are greyed out, an organisation policy owns this — contact an admin or use the PAT workaround below.

**Workaround: `RELEASE_PLEASE_TOKEN` repository secret**

1. Create a **fine‑grained** PAT scoped to **this repository**: **Contents** *Read and write*, **Issues** *Read and write* (release‑please may attach labels/cards), **Pull requests** *Read and write*.
2. Or a **classic** PAT with **`repo`** scope.
3. In the repo → **Settings** → **Secrets and variables** → **Actions** → **New repository secret** → name **`RELEASE_PLEASE_TOKEN`**, paste the PAT.

**.github/workflows/release.yml** passes **`RELEASE_PLEASE_TOKEN`** to release‑please when set (`secrets.RELEASE_PLEASE_TOKEN || github.token`).

### Commit discipline

Squash merges to **`main`** should keep a compliant message (`feat!: …`, `fix: …`). **Chores** (`chore:` / `deps:` …) bump the changelog too when included in the aggregated release—internal-only noise can stay **hidden** in config.

### First automation run

Existing prose above the newest dated **`## [v]`** heading is **not** parsed by release-please — notes come from commits on the Release PR. Keep the unpublished heading aligned with semver you expect next, **or** let the merged Release PR add a dated section and reconcile this file afterward.

---

## Nested `redis-pubsub` module (`redis-pubsub/v*` tags)

The **`redis-pubsub`** module is **not** wired into release-please ([nested modules](https://go.dev/wiki/Modules#publishing-multiple-modules-in-a-repository)). Publish it locally (Git tag **`redis-pubsub/v*`** pointing at **`main`** or another commit); **`.github/workflows/release.yml`** listens for matching tag **pushes** and runs **`proxy-warm`** for that submodule.

```bash
# Example — after committing on main you want tagged:
git tag redis-pubsub/v0.2.2 <sha>
git push origin refs/tags/redis-pubsub/v0.2.2
```

Create the **GitHub Release** for that tag in the UI when you publish (CI does not mint nested tags or Releases).

Consumers:

```bash
go get github.com/grx-gql/grx/redis-pubsub@v0.2.2
```

---

## Repo-wide housekeeping

### Checklist before merging an automated Release PR

- [ ] Green CI on **`main`**.
- [ ] Release PR description looks right (changelog + bump).
- [ ] Optionally update **`README`** or guides if behaviour changed.

After merge: **`go get github.com/grx-gql/grx@vX.Y.Z`**, **`pkg.go.dev`** may lag briefly.

---

## What we omit

No **GoReleaser** binaries — this repo is libraries only.
