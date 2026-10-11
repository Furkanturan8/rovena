#!/bin/sh
# Writes backend/.env from .env.example with random secrets. For CI and throwaway machines only;
# never overwrites an existing .env.
set -eu
cd "$(dirname "$0")/.."
if [ -e .env ]; then echo ".env already exists, not touching it" >&2; exit 0; fi
python3 - <<'PY'
import re, secrets
s = open(".env.example").read()
def fill(m):
    suffix = "Aa1!" if m.group(1) == "ADMIN_PASSWORD" else ""
    return f"{m.group(1)}={secrets.token_hex(24)}{suffix}"
s = re.sub(r"^((?:[A-Z]+_DB_PASSWORD|POSTGRES_PASSWORD|RABBITMQ_PASSWORD|JWT_SECRET|INTERNAL_API_KEY|ADMIN_PASSWORD))=.*$", fill, s, flags=re.M)
open(".env", "w").write(s)
PY
echo "wrote backend/.env"
