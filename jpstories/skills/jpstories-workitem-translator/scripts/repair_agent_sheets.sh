#!/usr/bin/env sh
# repair_agent_sheets.sh
# Repair and check completed jpstories translation sheets before import.
#
# Usage:
#   sh repair_agent_sheets.sh --story <story> [--stories-root stories]
#   sh repair_agent_sheets.sh --story <story> --file <sheet.txt> --check
#   sh repair_agent_sheets.sh --story <story> --file <sheet.txt> --quarantine-invalid
#   sh repair_agent_sheets.sh --source-sheet <agent.txt> --done-sheet <agent-done.txt> --rewrite-from-source
set -eu

PYTHON_BIN="${PYTHON:-}"
if [ -z "$PYTHON_BIN" ]; then
  if command -v python3 >/dev/null 2>&1; then
    PYTHON_BIN="python3"
  elif command -v python >/dev/null 2>&1; then
    PYTHON_BIN="python"
  else
    echo "python3 or python is required" >&2
    exit 127
  fi
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec "$PYTHON_BIN" "$SCRIPT_DIR/agent_sheet_tools.py" "$@"
