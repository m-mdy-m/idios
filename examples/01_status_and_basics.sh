#!/usr/bin/env bash
# Example 01 — Status check and basic wiring verification.
#
# Run this after ./scripts/bootstrap.sh to verify the install is healthy.
# Everything here works with zero models, zero network, zero API keys.
#
# Usage:
#   source .venv/bin/activate
#   bash examples/01_status_and_basics.sh

set -euo pipefail

echo "=== idios status ==="
idios status

echo ""
echo "=== Decision: new task with no context ==="
idios decide \
  --goal "understand transformer attention" \
  --task-state NEW

echo ""
echo "=== Decision: task with a weak concept ==="
idios decide \
  --goal "implement a transformer" \
  --task-state DECIDE \
  --weak-concept "self-attention" \
  --weak-concept "positional encoding"

echo ""
echo "=== Decision: task with project context ==="
idios decide \
  --goal "write the encoder block" \
  --project "mini-transformer" \
  --task-state PREPARE

echo ""
echo "=== Learning Core demo (zero AI) ==="
idios learn-demo

echo ""
echo "All examples completed successfully."
