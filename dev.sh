#!/usr/bin/env bash
set -euo pipefail
FASTFSO_ROOT="$(cd "$(dirname "$0")" && pwd)"
export FASTFSO_ROOT
cd "${FASTFSO_ROOT}/tools/dev"
exec go run . "$@"
