#!/usr/bin/env bash
set -e
KEY="${1:?api key required}"
BASE="${2:-http://127.0.0.1:9090}"
echo "== health =="
curl -sS -m 5 "$BASE/api/health"
echo
echo "== real streaming =="
curl -sN -m 120 "$BASE/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $KEY" \
  -d '{"model":"qwen-local","stream":true,"max_tokens":60,"messages":[{"role":"user","content":"只回答：1+1等于几"}]}' \
  | head -c 2000
echo
