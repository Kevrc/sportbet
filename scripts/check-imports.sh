#!/usr/bin/env bash
# Verifica las reglas de dependencias entre módulos del backend:
#   1. internal/<X> solo importa internal/<X> e internal/shared.
#   2. internal/shared nunca importa un módulo.
#   3. Ninguna capa domain importa pgx, chi ni net/http.
set -euo pipefail
cd "$(dirname "$0")/../backend"

MODULES="auth users admin events markets providers betting wallet settlement"
fail=0

for m in $MODULES; do
  bad=$(go list -f '{{join .Imports "\n"}}' "./internal/$m/..." | sort -u \
    | grep '^sportbet/internal/' | grep -v "^sportbet/internal/$m\(/\|$\)" | grep -v '^sportbet/internal/shared' || true)
  if [ -n "$bad" ]; then
    echo "✗ internal/$m importa otro módulo:"; echo "$bad" | sed 's/^/    /'; fail=1
  fi
done

pattern=$(echo "$MODULES" | tr ' ' '|')
bad=$(go list -f '{{join .Imports "\n"}}' ./internal/shared/... | sort -u | grep -E "^sportbet/internal/($pattern)(/|$)" || true)
if [ -n "$bad" ]; then
  echo "✗ internal/shared importa un módulo:"; echo "$bad" | sed 's/^/    /'; fail=1
fi

for m in $MODULES; do
  bad=$(go list -f '{{join .Imports "\n"}}' "./internal/$m/domain/..." | sort -u | grep -E '^(github.com/jackc/pgx|github.com/go-chi|net/http)' || true)
  if [ -n "$bad" ]; then
    echo "✗ internal/$m/domain importa infraestructura:"; echo "$bad" | sed 's/^/    /'; fail=1
  fi
done

[ "$fail" -eq 0 ] && echo "✓ reglas de importación OK"
exit "$fail"
