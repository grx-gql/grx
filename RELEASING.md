# Releasing grx

## Automatic releases (root module — default)

[`release-please`](https://github.com/googleapis/release-please) bumps the semver from [**Conventional Commits**](https://www.conventionalcommits.org/) merged to **`main`** (`feat:` / `fix:` / `perf:` → minor or patch depending on semver rules; `BREAKING CHANGE` footer or **`!`** → major).

**What happens**

1. **`Release Please`** (`.github/workflows/release-please.yml`) runs after every push to **`main`**.
2. It opens or updates **one Release PR**: bumps **`.release-please-manifest.json`**, rolls forwarded commits under **`CHANGELOG.md`** headings that match **`release-please-config.json`** (emoji‑prefixed sections like **`✨ Added`**, **`🐛 Fixed`**, **`💥 Breaking Changes`**, …), and refreshes GitHub-facing notes.
   - If you tweak the unpublished development heading in **`CHANGELOG.md`** manually (`## [x.y.z] - unpublished`), run **`make docs-changelog`** and commit **`CHANGELOG.md`** **and** **`docs/changelog.md`** together — CI (**`scripts/check-docs-changelog.sh`**) rejects drift.
3. **Merge that PR** → release-please writes the **`vMAJOR.MINOR.PATCH`** tag and opens the matching **GitHub Release**.
4. **Tag push** triggers **`.github/workflows/release.yml`** for **`go test`/vet**, **`go mod verify`**, and **proxy warmup** (`proxy.golang.org`).
5. **Documentation site builds** call **`make docs-content`**, which re-runs **`scripts/sync-changelog.sh`** against whatever revision is checked out — the committed **`docs/changelog.md`** file must mirror **`CHANGELOG.md`** at that SHA.

Configuration lives in **`release-please-config.json`** and **`.release-please-manifest.json`** (currently bootstrapped from the latest **`v*`** release tag).

### Commit discipline

Squash merges to **`main`** should keep a compliant message (`feat!: …`, `fix: …`). **Chores** (`chore:` / `deps:` …) bump the changelog too when included in the aggregated release—internal-only noise can stay **hidden** in config.

### First automation run

Existing prose above the newest dated **`## [v]`** heading is **not** parsed by release-please—notes come from commits on the Release PR. Keep the unpublished heading aligned with semver you expect next, **or** let the merged Release PR add a dated section and reconcile this file afterward.

---

## Semi-manual submodule (`redis-pubsub` nested Go module)

The **`redis-pubsub`** module is published with **prefix tags** **`redis-pubsub/v*`** (see [nested modules](https://go.dev/wiki/Modules#publishing-multiple-modules-in-a-repository)). It is **not** wired into release-please yet.

Use **Actions → Release → Run workflow** with:

| Field | Example |
|-------|---------|
| Tag | **`redis-pubsub/v0.2.1`** |
| Target | branch/commit (usually **`main`**) |

That flow creates/pushes the tag and a matching **GitHub Release** (`extract changelog` only works if **`CHANGELOG.md`** has a **`##`** section matching that semver).

Consumers:

```bash
go get github.com/grx-gql/grx/redis-pubsub@v0.2.1
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

No **GoReleaser** binaries—this repo is libraries only.
