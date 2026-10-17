#!/usr/bin/env bash
set -euo pipefail

mode="${1:-check}"
if [[ "$mode" != "check" && "$mode" != "write" ]]; then
  echo "usage: $0 [check|write]" >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

mapfile -t go_files < <(
  find . -type f -name '*.go' \
    -not -path './.git/*' \
    -not -path './node_modules/*' \
    -not -path './vendor/*' \
    -print | sort
)

if ((${#go_files[@]} > 0)); then
  gofmt_command=(gofmt)
  if [[ -f backend/go.mod ]]; then
    gofmt_command=("$(go -C backend env GOROOT)/bin/gofmt")
  fi

  if [[ "$mode" == "write" ]]; then
    "${gofmt_command[@]}" -w "${go_files[@]}"
  else
    unformatted="$("${gofmt_command[@]}" -l "${go_files[@]}")"
    if [[ -n "$unformatted" ]]; then
      echo "Go files require gofmt:" >&2
      echo "$unformatted" >&2
      exit 1
    fi
  fi
else
  echo "format: no Go files; skipping gofmt"
fi

prettier="$repo_root/node_modules/.bin/prettier"
if [[ ! -x "$prettier" ]]; then
  echo "format: Prettier is not installed; run npm ci" >&2
  exit 1
fi

prettier_targets=(
  "package.json"
  "package-lock.json"
  ".prettierrc.json"
  "schemas/**/*.json"
  "registry/**/*.json"
  "registry/**/*.{yaml,yml}"
  "examples/**/*.{json,yaml,yml}"
  ".github/**/*.{json,yaml,yml,js,mjs,cjs,ts}"
  "web/**/*.{json,yaml,yml,js,mjs,cjs,jsx,ts,tsx,css,scss}"
)

if [[ "$mode" == "write" ]]; then
  "$prettier" --write --ignore-unknown --no-error-on-unmatched-pattern "${prettier_targets[@]}"
else
  "$prettier" --check --ignore-unknown --no-error-on-unmatched-pattern "${prettier_targets[@]}"
fi
