#!/usr/bin/env bash
set -euo pipefail

go list ./... 2>/dev/null | grep -v '^github.com/duggal1/Sapphire-cli/internal/orchestration/agents-routing/'
