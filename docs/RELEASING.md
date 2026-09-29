# Releasing: one version per phase

The roadmap is organized in phase milestones (Phase 1, Phase 2, …). Each phase
ships as **one** library version, built from stacked PRs, instead of one
version per PR.

Versions are computed by [semantic-release](https://semantic-release.gitbook.io/)
from Conventional Commits (see `.releaserc.json`). It only runs on `master`
(`.github/workflows/release.yml`), so nothing is released until a phase reaches
`master`.

## Branches

| Branch | Purpose |
|---|---|
| `master` | Released code. Every push that contains `feat`/`fix`/`perf`/`refactor` commits produces a version. |
| `phase-N/main` | Integration branch for phase N, created from `master` when the phase starts. |
| `phase-N/<issue>-<slug>` | One branch per issue in the phase, for example `phase-2/19-zapslog`. |

CI (`.github/workflows/go.yml`) runs for pushes to `master` and `phase-*/main`,
and for PRs into `master` or any `phase-*/**` branch.

## Workflow

1. **Start the phase.** Create `phase-N/main` from the latest `master`.
2. **Stack the PRs.** Open one PR per issue:
   - The first PR targets `phase-N/main`.
   - Each following PR targets the branch of the PR below it.
   - Order the stack by dependency, as described in the issues and in the milestone description.
   - Title every PR as a Conventional Commit (`feat(zap): …`, `fix: …`). Link the issue with `Closes #N` and set the phase milestone.
3. **Merge the stack into the phase, bottom-up, with squash.**
   - Each issue becomes one Conventional Commit on `phase-N/main`.
   - After a PR merges and its branch is deleted, GitHub retargets the next PR to `phase-N/main`.
   - Keep the remaining branches updated by merging `phase-N/main` into them. Don't rebase shared branches.
4. **Open the phase PR** `phase-N/main` → `master`:
   - Title: `release: Phase N — <milestone name>`.
   - Assign the phase milestone. The release job uses it to name the version.
   - Body: the list of stacked PRs, the release gates from the milestone (for example migration notes or the consumer inventory), and the rollout plan.
5. **Merge the phase PR with a merge commit, not squash.**
   - semantic-release sees every commit from the phase and computes a single bump: major if any commit is breaking, otherwise minor if any `feat`, otherwise patch.
   - The release notes list every change.
6. **Automatic naming.** After semantic-release publishes `vX.Y.Z`, the `label-phase-release` job:
   - Renames the GitHub release to `vX.Y.Z — Phase N — <milestone name>`.
   - Adds links to the milestone and the phase PR.
7. **Roll out** following the Phase 5 plan (#28): the pilot service adopts the new version first.

A phase that only contains `docs`, `test`, `chore` or `ci` commits merges
without producing a version. That is expected, for example for a decision-only
phase.

## Changes outside a phase

Repository maintenance (`ci:`, `docs:`, `chore:`) can go straight to `master`
and does not produce a version. An urgent `fix:` merged directly to `master`
produces a patch release without a phase name. Use this only for hotfixes.

## Nested modules

Once the Zap adapter becomes its own module (#21), its tags are prefixed
(`zap/vX.Y.Z`). The details of how those tags are produced live with that
change.
