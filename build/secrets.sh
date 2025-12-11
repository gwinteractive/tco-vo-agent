#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${1:-}" ]]; then
  echo "OPENAI KEY missing" >&2
  exit 1
fi

if ! gcloud secrets describe "${OPENAI_KEY_NAME}" --project "${GOOGLE_PROJECT}" >/dev/null 2>&1; then
  gcloud secrets create "${OPENAI_KEY_NAME}" --replication-policy="automatic" --project "${GOOGLE_PROJECT}"
  printf "%s" "${1}" | gcloud secrets versions add "${OPENAI_KEY_NAME}" --data-file=- --project "${GOOGLE_PROJECT}" >/dev/null
fi
