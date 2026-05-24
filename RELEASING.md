# Releasing grx

## Automatic releases (root module)

[`release-please`](https://github.com/googleapis/release-please) bumps the semver from [**Conventional Commits**](https://www.conventionalcommits.org/) merged to **`main`** (`feat:` / `fix:` / `perf:` → minor or patch depending on semver rules; `BREAKING CHANGE` footer or **`!`** → major).

**What happens**

1. **release‑please** runs from **`.github/workflows/release.yml`** when **`main`** is updated **or** when you use **Actions → Release → Run workflow** with branch **`main`** (optional **workflow_dispatch** — same job as a push, no extra inputs). It opens or updates **one Release PR**: bumps **`.release-please-manifest.json`**, rolls commits under **`CHANGELOG.md`** headings that match **`release-please-config.json`** (emoji‑prefixed sections such as **`✨ Added`**, **`🐛 Fixed`**, **`💥 Breaking Changes`**, …).
   - If you tweak the unpublished development heading in **`CHANGELOG.md`** manually (`## [x.y.z] - unpublished`), run **`make docs-changelog`** and commit **`CHANGELOG.md`** **and** **`docs/changelog.md`** together — CI (**`scripts/check-docs-changelog.sh`**) rejects drift.
2. **Merge that Release PR** → release‑please creates the **`vMAJOR.MINOR.PATCH`** tag and **GitHub Release** for the **root** Go module (`github.com/grx-gql/grx`).
3. **Tag push** runs the same **`Release`** workflow: **`go test`/vet**, **`go mod verify`**, and **`proxy.golang.org`** warmup for whatever tag landed (`v*` root or **`redis-pubsub/v*`** nested).
4. **Documentation site builds** call **`make docs-content`**, which re-runs **`scripts/sync-changelog.sh`** against the checked-out revision — committed **`docs/changelog.md`** must mirror **`CHANGELOG.md`** at that SHA.

Configuration lives in **`release-please-config.json`** and **`.release-please-manifest.json`** (typically bootstrapped from the latest **`v*`** tag).

Versioning and tags for the **root** module come only from **release‑please** (Release PR merge creates **`v*`**); the workflow offers **manual dispatch only to re-run release‑please** on **`main`**, not ad-hoc tagging.

### Permissions

If **release‑please** fails with *GitHub Actions is not permitted to create or approve pull requests*, enable **Allow GitHub Actions to create and approve pull requests** under **Repository (or Organization) Settings → Actions → General → Workflow permissions**, or add repo secret **`RELEASE_PLEASE_TOKEN`**: a PAT with **`contents`** and **`pull-requests`** on this repository (otherwise the workflow uses **`GITHUB_TOKEN`**).

### Commit discipline

Squash merges to **`main`** should keep a compliant message (`feat!: …`, `fix: …`). **Chores** (`chore:` / `deps:` …) bump the changelog too when included in the aggregated release—internal-only noise can stay **hidden** in config.

### First automation run

Existing prose above the newest dated **`## [v]`** heading is **not** parsed by release-please — notes come from commits on the Release PR. Keep the unpublished heading aligned with semver you expect next, **or** let the merged Release PR add a dated section and reconcile this file afterward.

---

## Nested `redis-pubsub` module (`redis-pubsub/v*` tags)

The **`redis-pubsub`** module is **not** wired into release-please ([nested modules](https://go.dev/wiki/Modules#publishing-multiple-modules-in-a-repository)). Publish it locally (Git tag **`redis-pubsub/v*`** pointing at **`main`** or another commit); **`.github/workflows/release.yml`** listens for matching tag **pushes** and runs **`verify`** + **`proxy-warm`** for that submodule.

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
