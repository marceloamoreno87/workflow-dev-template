#!/usr/bin/env bash
set -euo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
if [[ $# -ne 1 || ! "$1" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]]; then
  echo 'Uso: new-spec.sh nome-da-feature (letras minúsculas, números e hífens)' >&2
  exit 2
fi
target="$root/specs/backlog/$1"
templates="$root/.specify/templates/overrides"
for file in spec plan tasks; do
  test -f "$templates/$file-template.md"
done
mkdir -p -- "$root/specs/backlog"
mkdir -- "$target"
for file in spec plan tasks; do
  sed "s/{{FEATURE}}/$1/g" "$templates/$file-template.md" > "$target/$file.md"
done
printf 'Spec criada: %s\n' "$target"
node "$root/.spec-kit/bin/feature.js" select "specs/backlog/$1"
