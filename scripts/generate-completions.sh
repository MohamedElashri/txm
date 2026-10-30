#!/usr/bin/env bash
set -euo pipefail
repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_dir"
mkdir -p completions
# Generation is backend-independent, including without CGO/native support.
for shell in bash zsh fish powershell; do
  case "$shell" in
    zsh) output=_txm ;;
    powershell) output=txm.ps1 ;;
    *) output="txm.$shell" ;;
  esac
  CGO_ENABLED=0 go run . completion "$shell" > "completions/$output"
done
