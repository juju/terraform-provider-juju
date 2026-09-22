---
name: code-review
description: Perform a senior-level code review of a pull request following the
  structured severity-graded format (verdict line, Critical/Warnings/Suggestions/
  Architecture/Looks Good). Use when asked to review a PR, a diff, proposed
  changes, or when verifying that code changes are ready to merge.
---

# Code Review Skill

Perform an expert-level review of a pull request: the diff, the PR description,
and enough surrounding repository context to reason about real behavior, not
just changed lines.

## Context gathering

Before writing any finding:

- Read the full diff of the changes under review.
- Read the PR description and checklist (if any). Its claims are inputs to
  verify, not facts to trust.
- Read enough surrounding repository context to reason about behavior:
  callers of changed functions, registration/wiring sites, test
  infrastructure and fixtures, domain services and state layers.
- Search the repo repo-wide when a change affects a producer/consumer pair,
  to check that all sites were updated consistently.
- Consult git history (`git log`, `git blame`) for the changed code when it
  clarifies prior intent or reveals drift between parallel copies.

## Review dimensions

Evaluate the change along each dimension. Findings may come from any
dimension; do not limit yourself to the diff hunks.

1. **Correctness and regressions.** Trace each changed code path end-to-end
   and compare against pre-change behavior. Flag behavior changes and
   regressions, especially ones that would only manifest in configurations
   or deployment states the tests do not cover (different caller types,
   upgrade paths, missing rows or data). For behavior changes, show the old
   behavior, the new behavior, and who is affected.
2. **PR description accuracy.** Verify claims made in the PR description and
   checklist against the actual diff. Call out unchecked checklist items that
   are not satisfied by the changes, and description claims the diff does not
   support.
3. **Tests.** Do the tests pin the new behavior? Are there gaps where new or
   changed behavior has no test that would fail if it regressed? Do test
   fixtures or fakes mask production semantics (granting things production
   would not, or vice versa) so tests pass for reasons that do not hold in
   production?
4. **Coverage.** Are all producers/callers of changed behavior updated
   consistently? Verify with a repo-wide search when applicable. Flag any
   site left in the old state.
5. **Compatibility.** Existing persisted data, migrations, and upgrade
   paths: does the change handle entities created before it, or does it only
   cover newly-created ones? If existing data is left behind, a migration is
   needed or the limitation must be stated explicitly.
6. **Quality.** Misleading names after a behavior change, redundant
   conditions (prove redundancy, do not just assert it), ordering of checks
   (authorization and validation before data fetches), missing fields
   (compared against sibling code paths), duplication that invites drift,
   unrelated drive-by changes that belong in a separate commit.
7. **Performance.** Query counts, round trips, and allocations per
   operation, compared against the old code. Flag regressions, especially
   multiplicative ones (per-item work inside loops).
8. **Architecture.** Alternative approaches and their tradeoffs, consistency
   of the change with similar sites in the codebase, and whether a better API
   shape would make the call sites simpler or safer.

## Severity classification

- **Critical**: a regression of existing behavior, a correctness bug, or a
  change that would break real users or deployments. Must be fixed, or
  explicitly justified in the PR description and pinned by a test.
- **Warnings**: should be fixed before merging — quality issues, test gaps,
  description inaccuracies, compatibility gaps.
- **Suggestions**: nice to have — performance improvements, deduplication,
  consistency polish, scope suggestions.

## Output format

Mandatory, in this exact order:

1. Verdict line:

   `Verdict: <Comment|Approve|Request changes> (N critical issues, N warnings, N suggestions)`

   Counts must match the sections that follow; omit zero categories.

2. `### Critical` — numbered findings. If empty, write `None.`
3. `### Warnings` — numbered findings. If empty, write `None.`
4. `### Suggestions` — numbered findings. If empty, write `None.`
5. `### Architecture and Alternative Approaches` — bullet (`•`) discussion of
   the chosen approach versus alternatives, with tradeoffs. Validate the
   chosen approach when it is right; do not invent alternatives.
6. `### Looks Good` — bullet (`•`) list of specific things done well,
   including in the PR description (e.g. honest accounting of regressions or
   query costs). Be specific: cite code and explain why it is right.

## Finding requirements

Every finding must:

- Cite `file:line-range` precisely, for files inside and outside the diff.
- For behavior changes: show old vs new behavior and who is affected.
- Propose a concrete fix, or state the alternative (e.g. "the PR description
  must say so and a test must pin it") when justification is acceptable.

## Tone

- Direct and technical. No praise padding outside the Looks Good section.
- Do not soften a regression because the PR has other merits.
- End the review with an assessment of PR scope: is it appropriately sized,
  or should it be split?
