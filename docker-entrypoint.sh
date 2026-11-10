#!/bin/sh
set -eu
# A new signing key on restart invalidates access tokens; refresh sessions remain in PostgreSQL.
export JWT_SECRET_BASE64="${JWT_SECRET_BASE64:-$(openssl rand -base64 32)}"
exec /app/api
