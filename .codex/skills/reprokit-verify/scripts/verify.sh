#!/usr/bin/env bash
set -euo pipefail

mode="${1:-quick}"
if [[ "$mode" != "quick" && "$mode" != "full" ]]; then
  echo "usage: $0 [quick|full]" >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "verify: formatting"
bash .codex/skills/reprokit-verify/scripts/format.sh check

echo "verify: whitespace"
git diff --check

if [[ -f go.mod ]]; then
  echo "verify: go vet"
  go vet ./...

  echo "verify: go test"
  go test ./...

  if [[ "$mode" == "full" ]]; then
    echo "verify: go race test"
    go test -race ./...
  fi
else
  echo "verify: no go.mod; skipping Go checks"
fi

if [[ -f web/package.json ]]; then
  if [[ ! -d web/node_modules ]]; then
    echo "verify: web/node_modules is missing; run npm --prefix web ci" >&2
    exit 1
  fi

  echo "verify: web lint/test/build when defined"
  npm --prefix web run lint --if-present
  npm --prefix web test --if-present
  npm --prefix web run build --if-present
else
  echo "verify: no web/package.json; skipping Web checks"
fi

if [[ "$mode" == "full" ]]; then
  if [[ "${REPROKIT_RUN_DOCKER_TESTS:-0}" == "1" ]]; then
    if [[ ! -f go.mod ]]; then
      echo "verify: Docker integration tests requested but go.mod is missing" >&2
      exit 1
    fi
    echo "verify: harmless Docker integration tests"
    go test -tags=integration ./...
  else
    echo "verify: Docker integration tests skipped; set REPROKIT_RUN_DOCKER_TESTS=1 to opt in"
  fi
fi

echo "verify: ${mode} checks passed"
