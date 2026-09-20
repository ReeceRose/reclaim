---
name: upgrade-deps
description: Upgrade every dependency in the repo (web/, landing/, go.mod) to latest including majors, migrate the Biome config, and gate the result on the full CI suite. On failure, isolates the culprit package, reverts only that one, and reports what was held back and why. Use when asked to update, upgrade, or bump dependencies, refresh the lockfiles, or get the repo onto current package versions.
---

# Upgrade dependencies

Move all three ecosystems to their latest versions, keep everything that
passes the CI gates, and roll back only what breaks.

The result is left **uncommitted in the working tree**. Do not commit, branch,
or push unless the user asks.

## Ecosystems

| Dir | Manifest | Upgrade | Install |
|---|---|---|---|
| `web/` | `web/package.json` + `pnpm-lock.yaml` | `pnpm update --latest` | pnpm, from `packageManager` |
| `landing/` | `landing/package.json` + `pnpm-lock.yaml` | `pnpm update --latest` | pnpm, from `packageManager` |
| root | `go.mod` + `go.sum` | `go get -u ./... && go mod tidy` | go |

`web/` and `landing/` are **independent** pnpm projects, not a workspace. Each
has its own lockfile and its own `biome.json`. Upgrade and gate them separately.

## Version pinning — no range prefixes

Every dependency in both `package.json` files is pinned to an **exact** version.
No `^`, no `~`, no `>=`. Every version this skill writes must stay that way.

There is no `.npmrc`, so pnpm's default save-prefix is `^` and any bare
`pnpm add` will reintroduce one. Always pass `-E` (`--save-exact`), or `-DE`
for a devDependency:

```bash
pnpm add -E next@16.3.4
pnpm add -DE @biomejs/biome@2.5.12
```

After every step that writes to a `package.json` — the upgrade, a Biome
migration, each revert — verify no prefix crept in:

```bash
grep -nE '"[^"]+": *"[~^>=<]' web/package.json landing/package.json
```

Any hit is a defect in the run: rewrite that entry to the bare version and
re-run `pnpm install` so the lockfile agrees. This check is cheap; run it
rather than assuming `pnpm update --latest` preserved the style.

## Gates

The acceptance bar is everything CI runs. Cheapest first, so a failure surfaces
before an expensive build:

```bash
make lint                # go vet + biome ci + tsc, both web/ and landing/
go test ./...
make test-race           # scanner, worker, jobs, api, store, notify
cd web && pnpm run build
cd landing && pnpm run build
```

`make lint` runs `-j3` in parallel, which interleaves output. When it fails,
re-run the specific target serially (`make lint-web`, `make lint-landing`,
`make lint-go`) to get a legible error before attributing it to a package.

CI and the Dockerfile both install with `pnpm install --frozen-lockfile`, never
a bare `pnpm install`. A plain install passing does not prove the frozen one
does — a pnpm major changes the lockfile's *shape*, not just its resolutions.
Whenever a lockfile or the pnpm version moved, add:

```bash
cd web && pnpm install --frozen-lockfile
cd landing && pnpm install --frozen-lockfile
```

If the run touched `Dockerfile` or the Node image it pins, the image build is a
gate too — and it must name the right target. **`release-prebuilt` does not
build the frontend stage**: it copies `web/out` from the build context, which is
why CI's `docker` job is cheap on pull requests. The stage that installs pnpm is
only reachable through `release`:

```bash
docker build --target frontend-build .   # the changed stage, ~1 min
docker build --target release .          # the full release path, incl. Go
```

CI reaches the second only on a `v*` tag push, so a break in the frontend stage
would otherwise surface during a release. Build it here instead.

Both need a running daemon. If there is none, say so in the report and mark the
Dockerfile change **unvalidated** rather than letting the other green gates
imply it passed. If BuildKit stalls on `resolve image config for
docker-image://docker.io/docker/dockerfile:1`, pre-pull it (`docker pull
docker/dockerfile:1`) and rebuild — that resolve goes through a different
network path than `docker pull` and can time out on its own.

One gate has a side effect to undo: `cd web && pnpm run build` rewrites the
tracked `web/next-env.d.ts`. Next 16 builds to `.next` while `next dev` builds
to `.next/dev`, so its two generated `/// <reference>` imports flip depending on
which command ran last, and the committed copy is whichever the last person ran.
That diff is not part of the upgrade — after the final build, run
`git checkout -- web/next-env.d.ts`, confirm `pnpm run type-check` still
passes, and leave the manifests as the only change.

Never launch a browser, Playwright, or any e2e automation to verify an upgrade.
The gates above are the verification. If a visual check is warranted, ask the
user to look in their own browser.

## Procedure

### 1. Preflight

- `git status --porcelain` must show no **tracked** modifications. If it does,
  stop and ask the user to commit or stash — the revert path is `git checkout --`
  on manifests, which would destroy unrelated work. Untracked entries (`??`) do
  not block: this skill's own `.agents/` directory is usually one of them, and
  `git checkout --` on a named manifest cannot reach them.
- Record `git rev-parse HEAD` in the report as the baseline.
- Snapshot the six manifest files so a revert never depends on the index:
  ```bash
  mkdir -p /tmp/reclaim-deps-backup
  cp web/package.json web/pnpm-lock.yaml landing/package.json \
     landing/pnpm-lock.yaml go.mod go.sum /tmp/reclaim-deps-backup/
  cp web/biome.json /tmp/reclaim-deps-backup/web-biome.json
  cp landing/biome.json /tmp/reclaim-deps-backup/landing-biome.json
  ```
  (`package.json` collides between the two dirs — back them up under distinct
  names or in per-dir subdirectories.)

### 2. Baseline the gates

Run the full gate suite **before** upgrading anything. A pre-existing failure
must be identified now, or step 5 will blame it on a package. If the baseline
is red, report exactly which gate fails and stop: upgrading on top of a broken
tree produces an unattributable result.

### 3. Record current versions

Capture what is about to move, so the report can state before → after:

```bash
cd web && pnpm outdated --format list || true
cd landing && pnpm outdated --format list || true
go list -u -m all
```

`pnpm outdated` exits non-zero when anything is outdated — that is success
here, not an error.

`go list -u -m all` prints the whole module graph, which here includes goose's
unused SQL drivers (grpc, pgx, moby, ydb, …). `go get -u ./...` will not move
them: they sit in neither `require` block and nothing under `./...` imports
them. They are not held back — they were never in the upgrade set — so keep
them out of the report entirely.

### 4. Upgrade

Do the three ecosystems in this order (Go is independent; web before landing
because web is the larger surface):

```bash
cd web      && pnpm update --latest
cd landing  && pnpm update --latest
go get -u ./... && go mod tidy
```

Then run the pinning check from above. `pnpm update --latest` normally keeps the
existing specifier style, but it is the single largest rewrite of these files in
the run and the one most worth confirming.

`go get -u ./...` does not touch the `go` directive or the toolchain version.
Leave both alone unless the user asks.

**Do not** touch the `packageManager` field on your own. If pnpm itself is
behind, note it in the report as a proposed change covering every location
below — bumping one alone silently desyncs local from CI.

| Location | How the pnpm version gets there |
|---|---|
| `web/package.json` → `packageManager` | authoritative; the Docker stage derives from it, as does corepack if the developer runs it locally |
| `landing/package.json` → `packageManager` | authoritative, and separate |
| `.github/workflows/ci.yml` | two `pnpm/action-setup` `version:` inputs; must equal `packageManager` or the action fails on the mismatch |
| `Dockerfile` frontend stage | *derived* — `npm install -g "$(node -p …packageManager…)"` reads the copied `web/package.json`, so it needs no edit |
| `pnpm-lock.yaml` → `packageManagerDependencies` | *derived* — pnpm ≥12 records itself; regenerated by `pnpm install` |

Node is pinned separately, in three places: `node-version` in both CI jobs and
`FROM node:<major>-alpine` in the Dockerfile.

**The Dockerfile deliberately does not use corepack.** Corepack is not
distributed with Node 25 or later — the Node TSC voted to stop shipping it, so
24 (Krypton, LTS) is the last major that bundles it. Rather than cap the image
at 24 forever, the frontend stage installs pnpm itself:

```dockerfile
COPY web/package.json web/pnpm-lock.yaml ./
RUN npm install -g "$(node -p 'require("./package.json").packageManager.split("+")[0]')"
```

That reads the version straight out of `packageManager`, so it is still one pin,
not a sixth — and the `split("+")` tolerates a corepack-style integrity suffix
(`pnpm@12.5.1+sha512…`) if one is ever added. Do not "simplify" it to a
hardcoded `pnpm@<version>`: that silently desyncs from `packageManager` on the
next bump, and nothing in the gate suite would catch it.

The ordering matters — the `COPY` must precede the install, since the version is
read out of the copied file. That costs one layer of cache on a dependency bump,
which is correct: a changed `packageManager` *should* reinstall pnpm.

Node's major is therefore a free choice now. Prefer the Active LTS.

### 5. Migrate the Biome config

If `@biomejs/biome` moved to a new major, its config schema moved with it. In
each of `web/` and `landing/`:

```bash
pnpm exec biome migrate --write
```

This rewrites `biome.json` — including the pinned `$schema` URL, which carries
the version (`https://biomejs.dev/schemas/2.5.12/schema.json`) and is otherwise
left stale and wrong. Confirm the `$schema` version matches the installed
version afterwards; if `migrate` did not update it, fix it by hand.

Then let Biome reformat under the new rules, since a new major routinely
changes formatting defaults and every one of those is a `biome ci` failure:

```bash
pnpm run format          # biome format --write .
pnpm exec biome check --write .
```

Treat the resulting diff as part of the upgrade. Read it before accepting it —
if a lint autofix changed behaviour rather than style, that is a finding for
the report, not something to wave through.

The two configs are near-identical but not identical: `web/biome.json` sets
`vcs.root: "../"` and `landing/biome.json` does not. Migrate each in place;
never copy one over the other.

### 6. Gate, isolate, revert

Run the full gate suite. If everything passes, go to step 7.

On failure, the goal is the **maximum passing upgrade set**, not a blanket
rollback. Isolate the culprit:

1. **Read the error first.** A type error naming `next/navigation`, a lint rule
   named in a `biome ci` diagnostic, or a Go compile error citing a module path
   usually identifies the package outright. Treat that as a hypothesis, not a
   conclusion.
2. **Test the hypothesis** by reverting just that package to its baseline
   version and re-running only the gate that failed:
   - npm: `pnpm add -E <pkg>@<old>`, or `-DE` for a devDependency. The exact
     flag is mandatory — see *Version pinning* above.
   - Go: `go get <module>@<old> && go mod tidy`.
3. **If the error does not name a package**, binary-search. Restore the backed-up
   manifest, re-apply half the upgrades, re-run the failing gate, recurse. For
   pnpm this means editing `package.json` to the chosen versions and running
   `pnpm install`; do not re-run `pnpm update --latest`, which would re-upgrade
   everything.
4. Related packages move as a unit. `react` + `react-dom` + `@types/react` +
   `@types/react-dom`, and `next` + its Biome `next` domain rules, are single
   decisions — reverting one of a pair produces a peer-dependency failure that
   looks like a second, fake culprit.
5. After each revert, **re-run the full suite**, not just the failing gate. A
   downgrade can break something that was passing.

Cap the search: if more than ~3 isolation rounds do not converge, stop, restore
the affected ecosystem to its backup, and report it as unresolved with what you
learned. An unbounded bisect over a Next.js major is not a good use of the run.

If a major upgrade needs real code changes (a renamed API, a removed export),
**do not write them**. Revert the package and put the required migration in the
report as a proposed change. This skill upgrades dependencies; it does not
refactor the app behind them.

For Next.js specifically: `web/AGENTS.md` warns that this is Next.js 16 with
breaking changes from prior versions. Read `node_modules/next/dist/docs/`
before concluding anything about a Next API.

### 7. Report

Always end with a written summary, whether or not everything passed:

**Upgraded** — grouped by ecosystem, `pkg 1.2.3 → 2.0.0`, majors flagged.

**Held back** — per package: the version it would have moved to, the gate that
failed, the actual error (quoted, trimmed), and a concrete proposed change to
unblock it — the API that moved and its replacement, the config key that was
renamed, the code that needs rewriting and where it lives. "Needs investigation"
is not a proposed change.

**Config changes** — what `biome migrate` rewrote, what reformatting touched
(file count is enough unless something is surprising), any new lint rule now
firing. Confirm the pinning check came back clean.

**Gate results** — each gate and its outcome, so the user can see the accepted
set is genuinely green rather than assumed green.

**Follow-ups** — pnpm/Node/Go toolchain bumps needing a coordinated CI edit, a
deprecated package with no successor, anything noticed but out of scope.

State plainly if the run ended with nothing upgraded. A clean revert reported
honestly is a successful run; a green claim over a skipped gate is not.

## Full revert

If the user asks to abandon the run, or the tree ends up in an unclear state:

```bash
cp /tmp/reclaim-deps-backup/go.mod /tmp/reclaim-deps-backup/go.sum .
# restore each package.json / pnpm-lock.yaml / biome.json to its own dir
cd web && pnpm install --frozen-lockfile
cd landing && pnpm install --frozen-lockfile
```

Then confirm with `git status --porcelain` that the tree is clean again, and
re-run `make lint` to prove the restore is sound. `git checkout -- <paths>` is
equivalent and simpler when the backup and HEAD agree — but only ever name the
manifest paths explicitly, never `git checkout -- .`.
