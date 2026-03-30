#!/usr/bin/env bash
set -euo pipefail

pkgs="$(go list ./... 2>/dev/null || true)"
filtered="$(printf '%s\n' "$pkgs" | awk 'NF && $0 !~ /^github.com\/duggal1\/Sapphire-cli\/internal\/orchestration\/agents-routing\//')"
filtered="$(printf '%s\n' "$filtered" | awk 'NF && $0 !~ /^github.com\/duggal1\/Sapphire-cli\/internal\/orchestration\/persistent-memory/')"

if [[ -z "$filtered" ]]; then
  go list .
  exit 0
fi

printf '%s\n' "$filtered"
