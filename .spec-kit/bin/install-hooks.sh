#!/usr/bin/env bash
set -euo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cd -- "$root"
custom="$(git config --get core.hooksPath || true)"
if [[ -n "$custom" ]]; then
  echo "core.hooksPath já configurado ($custom). Integre .spec-kit/hooks/post-commit nesse gerenciador." >&2
  exit 1
fi
hooks="$(git rev-parse --git-path hooks)"
target="$hooks/post-commit"
if [[ -e "$target" || -L "$target" ]]; then
  if cmp -s -- "$root/.spec-kit/hooks/post-commit" "$target"; then
    chmod +x -- "$target"
    echo 'Hook já instalado.'
    exit 0
  fi
  if [[ -f "$target" ]] && grep -q '^# spec-kit OKF post-commit v1$' "$target"; then
    cp -- "$root/.spec-kit/hooks/post-commit" "$target"
    chmod +x -- "$target"
    echo 'Hook gerenciado atualizado.'
    exit 0
  fi
  echo "Hook existente preservado: $target. Integre-o manualmente antes de instalar." >&2
  exit 1
fi
mkdir -p -- "$hooks"
cp -- "$root/.spec-kit/hooks/post-commit" "$target"
chmod +x -- "$target"
echo "Hook instalado: $target"
