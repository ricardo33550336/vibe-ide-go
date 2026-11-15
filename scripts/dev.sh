#!/usr/bin/env bash
# Runs Vibe Dev with live reload (go tool mygo dev) beside the everyday
# Vibe. MyGo gives it the identifier dev.vibe.editor.dev, so its own
# instance lock; VIBE_DATA_DIR gives it its own settings and window state.
# Usage: scripts/dev.sh
# VIBE_DATA_DIR picks another data folder (default: Vibe-dev beside Vibe's).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export VIBE_DATA_DIR="${VIBE_DATA_DIR:-$HOME/Library/Application Support/Vibe-dev}"

# MyGo creates its own data folder, but not one set with SetPath.
mkdir -p "$VIBE_DATA_DIR"

echo "==> mygo dev (data in $VIBE_DATA_DIR)"
cd "$ROOT"
exec go tool mygo dev
