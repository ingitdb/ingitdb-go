#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="${1:-ingitdb}"

firebase deploy --only hosting:ingitdb-com --project "$PROJECT_ID" --config ../firebase.json
