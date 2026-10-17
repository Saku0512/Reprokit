---
name: reprokit-verify
description: Format and verify Reprokit changes. Use when running tests, checking formatting, diagnosing a failed project check, or preparing a change for review.
---

# Reprokit Verify

Use the repository scripts as the single verification interface. Do not substitute global formatter versions or invent commands from another project.

## Select the check

- Documentation-only or configuration-only change: run `npm run format:check` and `git diff --check`.
- Ordinary implementation change: run `npm run verify` during development.
- Core, Runtime, Security, shared-contract, dependency, or pre-PR verification: run `npm run verify:full`.
- Run a narrower package test first when diagnosing a failure, then rerun the applicable repository command.

If `node_modules` is missing or `package-lock.json` changed, run `npm ci` before verification. Do not use a globally installed Prettier.

## Workflow

1. Inspect the changed paths and choose the smallest applicable check above.
2. Run `npm run format` only when formatting changes are authorized; otherwise start with `npm run format:check`.
3. Run the selected verification command.
4. Fix failures caused by the current change, then rerun the failed check.
5. Report commands run, pass/fail status, and every skipped or unavailable check.

## Safety boundaries

- General verification must not start real vulnerable Vulhub labs.
- Docker integration tests require `REPROKIT_RUN_DOCKER_TESTS=1` and must use harmless dedicated fixtures.
- Never weaken a test, formatter, schema, or security rule merely to obtain a passing run.
- Do not modify unrelated files to resolve a pre-existing failure.
- A missing component is `skipped`, not `passed`.

## Script behavior

- `scripts/format.sh check|write` applies `gofmt` from `backend/go.mod` to Go and the pinned Prettier to supported repository files.
- `scripts/verify.sh quick|full` checks formatting and whitespace, then runs available Go checks from `backend/` and Web checks from `web/`. `full` adds the race suite and opted-in harmless Docker integration tests.
