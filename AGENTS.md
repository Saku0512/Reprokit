# Reprokit repository guidance

## Product and scope

Reprokit is a B2B security-training platform for application developers. It turns vulnerabilities reported in real products into reviewed lessons that cover reproduction, observation, root cause, remediation, and prevention. CVE IDs are evidence, not the primary learner-facing navigation.

Vulhub and compatible labs provide environments. Reprokit owns the lesson design, safe resolution and execution, learner workflow, and training administration. Do not add arbitrary-target scanning, arbitrary command execution, or automatic execution of unreviewed code.

## Read the context needed for the task

- Use [docs/b2b-security-training-requirements.md](docs/b2b-security-training-requirements.md) for product scope and priorities.
- Use [docs/b2b-domain-contract-v0.1.md](docs/b2b-domain-contract-v0.1.md) when changing domain entities, Lesson-to-Lab references, UI/Runner contracts, or code ownership.
- Use [docs/security-v1.md](docs/security-v1.md), [docs/runtime-v1.md](docs/runtime-v1.md), and [docs/lockfile-v1.md](docs/lockfile-v1.md) when changing Lab execution or trust boundaries.
- Use [docs/development-testing-v1.md](docs/development-testing-v1.md) when changing toolchains, CI, or test policy.

Do not read every design document for a narrow change. Existing implementation and schemas remain the source of truth where older documents conflict.

## Ownership and parallel work

- `@Saku0512` owns Core/Runner work: `backend/cmd/reprokit/`, `backend/internal/{lab,runtime,security,manifest,lockfile,registry}/`, `registry/`, and Manifest/Lock schemas.
- `@yui-szk` owns Learning/Training work: `web/`, `lessons/`, `backend/internal/lesson/`, Lesson schemas, and `docs/training/`.
- Shared contracts, `backend/go.mod`, `backend/go.sum`, public CLI/API surfaces, `backend/internal/training/`, CI, and root configuration require coordination and a small isolated PR.
- Keep one Issue per branch and include the Issue number when one exists. Avoid unrelated cleanup in implementation PRs.
- Split commits into small, purpose-focused units so each commit can be reviewed, tested, and reverted independently.
- Never merge a pull request automatically. After creating a PR and reporting its verification results, wait for the user's explicit merge instruction.
- Do not silently change a contract owned by the other development lane. Update the shared contract first and request the other owner's review.

## Commit messages

Follow [CONTRIBUTING.md](CONTRIBUTING.md#コミットメッセージとprタイトル) for commit subjects and PR titles. Use `<prefix>(<component>): <message>`, for example `test(web): 受講画面の回帰テストを拡充`.

## Lab images and Docker Hub

- Manage Lab image build definitions under `images/CVE-YYYY-NNNN/<product-version>/`; keep vulnerable and fixed versions separate. See [images/README.md](images/README.md) for provenance and published images.
- Publish Reprokit-managed images to Docker Hub under `saku0512/reprokit_cve-yyyy-nnnn`. Docker image repository names must be lowercase. Use version and source commit in human-readable tags, for example `2.63.5-8c7e9fd9a4ec`.
- Build for the declared platform from a fixed source revision. Record the source commit, platform, image digest, and relevant build limitations; verify public images are anonymously accessible. Do not overwrite an existing tag without an explicit request.
- Pin executable Compose references to the reviewed `@sha256:...` digest, not a mutable tag. Publishing an image does not by itself update a fixed Vulhub fork revision or make a Lab approved and runnable; update the fixed source and Registry only after the required human review.
- Docker Hub publication is an external write: perform it when requested, keep credentials out of the repository and output, and do not remove previously published images unless explicitly requested.

## Formatting

The repository formatter contract is deterministic and shared by both developers.

- Go: `gofmt` from the toolchain selected by `backend/go.mod`; do not use `gofumpt` or editor-specific alternatives.
- JSON, YAML, JavaScript, TypeScript, CSS, and future Web files: the repository-pinned Prettier version.
- Markdown under `docs/` is intentionally excluded from automatic formatting to avoid broad prose churn. Preserve the surrounding document style.
- Write source-code comments in Japanese.
- Editor settings must not override repository output. `.editorconfig` defines whitespace defaults, but `gofmt` and Prettier are authoritative.

Install formatter dependencies with `npm ci`. Use:

```bash
npm run format
npm run format:check
```

Never hand-edit `package-lock.json`; regenerate it with npm.

## Testing and verification

Use the project skill at `.codex/skills/reprokit-verify/SKILL.md` whenever a task asks to format, test, verify, prepare a PR, or diagnose a project check.

```bash
npm run verify
npm run verify:full
```

- Run focused checks while developing and the appropriate broader check before handoff.
- Documentation-only changes do not require unavailable application tests; still run formatting checks and `git diff --check`.
- Never claim a skipped or unavailable check passed. Report it as skipped with the reason.
- General verification must not start real vulnerable Vulhub labs. Docker integration tests must use harmless dedicated fixtures and explicit opt-in.
- Fix failures caused by the requested change and rerun the affected checks. Do not broaden the task to unrelated pre-existing failures.

## Security invariants

- `create`/prepare must not start containers.
- Resolve mutable sources to immutable revisions and verify Manifest, Lockfile, Compose, referenced files, and image digests.
- Bind published Lab ports to loopback and reject privileged, host-networked, Docker-socket, unsafe mount, or equivalent configurations.
- UI and CLI must call the same Go Core safety checks. Browser input must never become a shell command, arbitrary Compose path, image, repository, or target host.
- AI-assisted imports remain drafts until a human reviews sources, licensing, configuration, and safety. Unreviewed artifacts cannot enter the runnable Registry.
