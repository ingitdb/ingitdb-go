#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="${1:-ingitdb}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

firebase deploy --only hosting:ingitdb-com --project "$PROJECT_ID" --config "$ROOT_DIR/server/firebase.json"
