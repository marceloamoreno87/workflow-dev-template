#!/usr/bin/env bash
set -euo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
if [[ $# -gt 1 || ( $# -eq 1 && ! "$1" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ) ]]; then
  echo 'Uso: fast-track.sh [id-em-kebab-case]' >&2
  exit 2
fi
id="fast-track-${1:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
target="$root/specs/active/$id"
for file in spec plan tasks; do
  test -f "$root/.spec-kit/templates/fast-track-$file-template.md"
done
if [[ -e "$target" || -e "$target.md" || -L "$target.md" ]]; then
  echo "Fast-track já existe: $id" >&2
  exit 1
fi
if find "$root/specs/active" -mindepth 1 -maxdepth 1 -type d -print -quit 2>/dev/null | grep -q .; then
  echo 'Já existe uma feature ativa; arquive-a antes de abrir um fast-track.' >&2
  exit 1
fi
mkdir -p -- "$root/specs/active"
mkdir -- "$target"
for file in spec plan tasks; do
  sed "s/{{FEATURE}}/$id/g" "$root/.spec-kit/templates/fast-track-$file-template.md" > "$target/$file.md"
done
ln -s "$id/spec.md" "$target.md"
node "$root/.spec-kit/bin/feature.js" select "specs/active/$id"
printf 'Fast-track: %s.md\nPreencha um plano curto e registre aprovação antes de alterar src/.\n' "$target"
