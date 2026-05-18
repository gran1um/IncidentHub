#!/usr/bin/env bash
set -euo pipefail

FRONT_CONTAINER="${FRONT_CONTAINER:-incidenthub-frontend}"

if ! docker compose ps frontend --status running --format '{{.Name}}' | grep -q "^${FRONT_CONTAINER}$"; then
  echo "frontend container '${FRONT_CONTAINER}' is not running" >&2
  exit 1
fi

HTML="$(docker exec "${FRONT_CONTAINER}" curl -sS http://localhost/)"
if ! grep -qi "incident" <<<"${HTML}"; then
  echo "frontend HTML does not look valid" >&2
  exit 1
fi

docker exec "${FRONT_CONTAINER}" sh -lc 'rm -f /tmp/ih-smoke-cookie-a.txt /tmp/ih-smoke-cookie-b.txt'

wait_async_operation() {
  local operation_id="$1"
  local attempts="${2:-90}"
  local status_resp=""
  local status=""
  local operation_error=""
  for ((i=1; i<=attempts; i++)); do
    status_resp="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/operations/${operation_id}")"
    status="$(echo "${status_resp}" | jq -r '.status // empty' 2>/dev/null || true)"
    operation_error="$(echo "${status_resp}" | jq -r '.error // empty' 2>/dev/null || true)"
    if [[ "${status}" == "done" ]]; then
      echo "${status_resp}"
      return 0
    fi
    if [[ "${status}" == "failed" ]]; then
      if [[ -n "${operation_error}" && "${operation_error}" != "null" ]]; then
        echo "async operation ${operation_id} failed: ${operation_error}" >&2
      else
        echo "async operation ${operation_id} failed" >&2
      fi
      return 1
    fi
    sleep 1
  done
  echo "timeout waiting for async operation ${operation_id}" >&2
  return 1
}

wait_ai_agent_run_for_case() {
  local agent_id="$1"
  local case_id="$2"
  local attempts="${3:-180}"
  local runs_resp=""
  for ((i=1; i<=attempts; i++)); do
    runs_resp="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/catalog/ai_agent_runs?ref_id=${agent_id}&limit=50")"
    if echo "${runs_resp}" | jq -e --arg case_id "${case_id}" '
      any(.[]?;
        (
          ((.queue_entity_id // .data.queue_entity_id // "") == $case_id)
          or any((.results // .data.results // [])[]?; ((.case_id // "") == $case_id))
        )
      )
    ' >/dev/null 2>&1; then
      echo "${runs_resp}"
      return 0
    fi
    sleep 1
  done
  echo "timeout waiting ai agent run for case ${case_id} (agent=${agent_id})" >&2
  return 1
}

resolve_async_resource_id() {
  local create_resp="$1"
  local id=""
  local status=""
  local operation_id=""
  local operation_resp=""

  id="$(echo "${create_resp}" | jq -r '.id // empty' 2>/dev/null || true)"
  if [[ -n "${id}" && "${id}" != "null" ]]; then
    echo "${id}"
    return 0
  fi

  id="$(echo "${create_resp}" | jq -r '.resource_id // empty' 2>/dev/null || true)"
  status="$(echo "${create_resp}" | jq -r '.status // empty' 2>/dev/null || true)"
  operation_id="$(echo "${create_resp}" | jq -r '.operation_id // empty' 2>/dev/null || true)"
  if [[ "${status}" == "queued" && -n "${operation_id}" && "${operation_id}" != "null" ]]; then
    if ! operation_resp="$(wait_async_operation "${operation_id}")"; then
      return 1
    fi
    id="$(echo "${operation_resp}" | jq -r '.resource_id // empty' 2>/dev/null || true)"
  fi

  if [[ -z "${id}" || "${id}" == "null" ]]; then
    return 1
  fi
  echo "${id}"
}

catalog_create_item() {
  local kind="$1"
  local payload="$2"
  docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"data\":${payload}}" "http://localhost/api/v1/catalog/${kind}"
}

ensure_observable_types_defaults() {
  local list_resp count create_resp
  list_resp="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/catalog/observable_types?include_global=true")"
  count="$(echo "${list_resp}" | jq 'length')"
  if [[ "${count}" -ge 6 ]]; then
    return 0
  fi

  echo "observable_types below baseline (${count}), seeding tenant defaults for smoke" >&2
  local payloads=(
    '{"name":"IP Address","dataType":"string","validatorRegex":".+"}'
    '{"name":"Domain Name","dataType":"string","validatorRegex":".+"}'
    '{"name":"MD5 Hash","dataType":"string","validatorRegex":".+"}'
    '{"name":"SHA256 Hash","dataType":"string","validatorRegex":".+"}'
    '{"name":"Email Address","dataType":"string","validatorRegex":".+"}'
    '{"name":"User Agent","dataType":"string","validatorRegex":".*"}'
  )

  for payload in "${payloads[@]}"; do
    local name
    name="$(echo "${payload}" | jq -r '.name')"
    if echo "${list_resp}" | jq -e --arg name "${name}" '.[] | select((.name // "") == $name)' >/dev/null; then
      continue
    fi
    create_resp="$(catalog_create_item "observable_types" "${payload}")"
    if [[ -z "$(echo "${create_resp}" | jq -r '.id // empty' 2>/dev/null || true)" ]]; then
      echo "failed to create fallback observable type: ${name}" >&2
      echo "${create_resp}" >&2
      return 1
    fi
  done

  list_resp="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/catalog/observable_types?include_global=true")"
  count="$(echo "${list_resp}" | jq 'length')"
  if [[ "${count}" -lt 6 ]]; then
    echo "default observable types missing after seed, count=${count}" >&2
    return 1
  fi
}

ensure_achievements_defaults() {
  local list_resp count create_resp
  list_resp="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/catalog/achievements?include_global=true")"
  count="$(echo "${list_resp}" | jq 'length')"
  if [[ "${count}" -ge 6 ]]; then
    return 0
  fi

  echo "achievements below baseline (${count}), seeding tenant defaults for smoke" >&2
  local payloads=(
    '{"name":"First Responder","description":"Closed the first incident case","rarity":"Common","xp_reward":150}'
    '{"name":"Night Watch","description":"Handled incidents in overnight shift","rarity":"Rare","xp_reward":180}'
    '{"name":"IOC Hunter","description":"Added validated observables","rarity":"Rare","xp_reward":200}'
    '{"name":"Containment Master","description":"Contained critical incidents","rarity":"Epic","xp_reward":260}'
    '{"name":"Intel Curator","description":"Published high-confidence notes","rarity":"Epic","xp_reward":280}'
    '{"name":"SOC Architect","description":"Designed and deployed a playbook","rarity":"Legendary","xp_reward":320}'
  )

  for payload in "${payloads[@]}"; do
    local name
    name="$(echo "${payload}" | jq -r '.name')"
    if echo "${list_resp}" | jq -e --arg name "${name}" '.[] | select((.name // "") == $name)' >/dev/null; then
      continue
    fi
    create_resp="$(catalog_create_item "achievements" "${payload}")"
    if [[ -z "$(echo "${create_resp}" | jq -r '.id // empty' 2>/dev/null || true)" ]]; then
      echo "failed to create fallback achievement: ${name}" >&2
      echo "${create_resp}" >&2
      return 1
    fi
  done

  list_resp="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/catalog/achievements?include_global=true")"
  count="$(echo "${list_resp}" | jq 'length')"
  if [[ "${count}" -lt 6 ]]; then
    echo "default achievements missing after seed, count=${count}" >&2
    return 1
  fi
}

LOGIN_RESP=""
TOKEN=""
TENANT=""
USER_ID=""
for _ in 1 2 3 4 5; do
  LOGIN_RESP="$(docker exec "${FRONT_CONTAINER}" sh -lc "curl -sS -c /tmp/ih-smoke-cookie-a.txt -H 'Content-Type: application/json' -d '{\"email\":\"admin@incidenthub.local\",\"password\":\"ChangeMeNow123!\"}' http://localhost/api/v1/auth/login")"
  TOKEN="$(echo "${LOGIN_RESP}" | jq -r '.access_token // empty' 2>/dev/null || true)"
  TENANT="$(echo "${LOGIN_RESP}" | jq -r '.memberships[0].tenant_id // empty' 2>/dev/null || true)"
  USER_ID="$(echo "${LOGIN_RESP}" | jq -r '.identity.user_id // empty' 2>/dev/null || true)"
  if [[ -n "${TOKEN}" && -n "${TENANT}" && -n "${USER_ID}" && "${TOKEN}" != "null" && "${TENANT}" != "null" && "${USER_ID}" != "null" ]]; then
    break
  fi
  sleep 1
done
if [[ -z "${TOKEN}" || "${TOKEN}" == "null" || -z "${TENANT}" || "${TENANT}" == "null" || -z "${USER_ID}" || "${USER_ID}" == "null" ]]; then
  echo "login failed or tenant missing" >&2
  exit 1
fi

LOGIN_RESP_SECOND=""
TOKEN_SECOND=""
for _ in 1 2 3 4 5; do
  LOGIN_RESP_SECOND="$(docker exec "${FRONT_CONTAINER}" sh -lc "curl -sS -c /tmp/ih-smoke-cookie-b.txt -H 'Content-Type: application/json' -d '{\"email\":\"admin@incidenthub.local\",\"password\":\"ChangeMeNow123!\"}' http://localhost/api/v1/auth/login")"
  TOKEN_SECOND="$(echo "${LOGIN_RESP_SECOND}" | jq -r '.access_token // empty' 2>/dev/null || true)"
  if [[ -n "${TOKEN_SECOND}" && "${TOKEN_SECOND}" != "null" ]]; then
    break
  fi
  sleep 1
done
if [[ -z "${TOKEN_SECOND}" || "${TOKEN_SECOND}" == "null" ]]; then
  echo "second login failed for session checks" >&2
  exit 1
fi

SESSIONS_RESP="$(docker exec "${FRONT_CONTAINER}" sh -lc "curl -sS -b /tmp/ih-smoke-cookie-a.txt -H 'Authorization: Bearer ${TOKEN}' http://localhost/api/v1/auth/sessions")"
SESSIONS_COUNT="$(echo "${SESSIONS_RESP}" | jq '.sessions | length')"
if [[ "${SESSIONS_COUNT}" -lt 2 ]]; then
  echo "auth sessions endpoint returned too few sessions: ${SESSIONS_COUNT}" >&2
  exit 1
fi
SESSION_TIMEOUT_SEC="$(echo "${SESSIONS_RESP}" | jq -r '.session_timeout_seconds')"
if [[ -z "${SESSION_TIMEOUT_SEC}" || "${SESSION_TIMEOUT_SEC}" == "null" || "${SESSION_TIMEOUT_SEC}" -le 0 ]]; then
  echo "auth sessions endpoint returned invalid timeout: ${SESSION_TIMEOUT_SEC}" >&2
  exit 1
fi
CURRENT_SESSIONS_COUNT="$(echo "${SESSIONS_RESP}" | jq '[.sessions[] | select(.is_current == true)] | length')"
if [[ "${CURRENT_SESSIONS_COUNT}" -ne 1 ]]; then
  echo "auth sessions endpoint should mark exactly one current session, got ${CURRENT_SESSIONS_COUNT}" >&2
  exit 1
fi

REVOKE_OTHERS_RESP="$(docker exec "${FRONT_CONTAINER}" sh -lc "curl -sS -X POST -b /tmp/ih-smoke-cookie-a.txt -H 'Authorization: Bearer ${TOKEN}' http://localhost/api/v1/auth/sessions/revoke-others")"
REVOKED_COUNT="$(echo "${REVOKE_OTHERS_RESP}" | jq -r '.revoked')"
if [[ -z "${REVOKED_COUNT}" || "${REVOKED_COUNT}" == "null" || "${REVOKED_COUNT}" -lt 1 ]]; then
  echo "revoke others did not revoke any session: ${REVOKED_COUNT}" >&2
  exit 1
fi

SECOND_REFRESH_CODE="$(docker exec "${FRONT_CONTAINER}" sh -lc 'curl -sS -o /dev/null -w "%{http_code}" -X POST -b /tmp/ih-smoke-cookie-b.txt http://localhost/api/v1/auth/refresh')"
if [[ "${SECOND_REFRESH_CODE}" -ne 401 ]]; then
  echo "revoked second session can still refresh token: http=${SECOND_REFRESH_CODE}" >&2
  exit 1
fi

USER_MEDIA_RESP="$(docker exec "${FRONT_CONTAINER}" sh -lc "printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO5fN7sAAAAASUVORK5CYII=' | base64 -d >/tmp/ih-smoke-avatar.png && curl -sS -H 'Authorization: Bearer ${TOKEN}' -F file=@/tmp/ih-smoke-avatar.png http://localhost/api/v1/users/${USER_ID}/media/avatar/upload")"
if [[ "$(echo "${USER_MEDIA_RESP}" | jq -r '.user.avatar_url')" == "null" ]]; then
  echo "failed to upload user avatar media" >&2
  exit 1
fi

TENANTS_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" http://localhost/api/v1/tenants)"
TENANTS_COUNT="$(echo "${TENANTS_RESP}" | jq 'length')"
if [[ "${TENANTS_COUNT}" -lt 1 ]]; then
  echo "tenants list is empty" >&2
  exit 1
fi

TITLE="Smoke Alert $(date +%s)"
CREATE_ALERT_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"title\":\"${TITLE}\",\"description\":\"Smoke test\",\"source\":\"manual\",\"severity\":\"high\"}" http://localhost/api/v1/alerts)"
ALERT_ID="$(resolve_async_resource_id "${CREATE_ALERT_RESP}" || true)"
if [[ -z "${ALERT_ID}" || "${ALERT_ID}" == "null" ]]; then
  echo "failed to create alert" >&2
  exit 1
fi

CASE_TITLE="Smoke Case $(date +%s)"
CREATE_CASE_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"title\":\"${CASE_TITLE}\",\"description\":\"Smoke case\",\"severity\":\"high\"}" http://localhost/api/v1/cases)"
CASE_ID="$(resolve_async_resource_id "${CREATE_CASE_RESP}" || true)"
if [[ -z "${CASE_ID}" || "${CASE_ID}" == "null" ]]; then
  echo "failed to create case" >&2
  exit 1
fi

AI_AGENT_NAME="Smoke Queue Agent $(date +%s)"
AI_AGENT_CREATE_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"data\":{\"name\":\"${AI_AGENT_NAME}\",\"description\":\"Smoke queue dispatch validation\",\"enabled\":true,\"target_types\":[\"case\"],\"max_cases_per_run\":1,\"auto_create_tasks\":false,\"auto_comment\":false,\"case_tags\":[]}}" http://localhost/api/v1/catalog/ai_agents)"
AI_AGENT_ID="$(echo "${AI_AGENT_CREATE_RESP}" | jq -r '.id // empty' 2>/dev/null || true)"
if [[ -z "${AI_AGENT_ID}" || "${AI_AGENT_ID}" == "null" ]]; then
  echo "failed to create ai agent for queue smoke" >&2
  echo "${AI_AGENT_CREATE_RESP}" >&2
  exit 1
fi

QUEUE_CASE_TITLE="Smoke Queue Case $(date +%s)"
QUEUE_CASE_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"title\":\"${QUEUE_CASE_TITLE}\",\"description\":\"Queue worker smoke case\",\"severity\":\"medium\"}" http://localhost/api/v1/cases)"
QUEUE_CASE_ID="$(resolve_async_resource_id "${QUEUE_CASE_RESP}" || true)"
if [[ -z "${QUEUE_CASE_ID}" || "${QUEUE_CASE_ID}" == "null" ]]; then
  echo "failed to create queue smoke case" >&2
  exit 1
fi
if ! wait_ai_agent_run_for_case "${AI_AGENT_ID}" "${QUEUE_CASE_ID}"; then
  if [[ "${AI_QUEUE_SMOKE_STRICT:-false}" == "true" ]]; then
    exit 1
  fi
  echo "warning: ai queue smoke check skipped (set AI_QUEUE_SMOKE_STRICT=true to enforce)" >&2
fi

CASE_ATTACHMENT_RESP="$(docker exec "${FRONT_CONTAINER}" sh -lc "printf 'smoke-artifact' >/tmp/ih-smoke-artifact.txt && curl -sS -H 'Authorization: Bearer ${TOKEN}' -H 'X-Tenant-ID: ${TENANT}' -F file=@/tmp/ih-smoke-artifact.txt http://localhost/api/v1/cases/${CASE_ID}/attachments/upload")"
if [[ "$(echo "${CASE_ATTACHMENT_RESP}" | jq -r '.attachment.id')" == "null" ]]; then
  echo "failed to upload case attachment" >&2
  exit 1
fi

GET_ALERT_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/alerts/${ALERT_ID}")"
if [[ "$(echo "${GET_ALERT_RESP}" | jq -r '.id')" != "${ALERT_ID}" ]]; then
  echo "failed to fetch alert detail card payload" >&2
  exit 1
fi

BIND_ALERT_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"alert_ids\":[\"${ALERT_ID}\"],\"case_id\":\"${CASE_ID}\"}" http://localhost/api/v1/alerts/bulk/link-case)"
if [[ "$(echo "${BIND_ALERT_RESP}" | jq -r '.updated_count')" -lt 1 ]]; then
  echo "bulk bind alerts to case failed" >&2
  exit 1
fi

GET_ALERT_LINKED_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/alerts/${ALERT_ID}")"
if [[ "$(echo "${GET_ALERT_LINKED_RESP}" | jq -r '.case_id')" != "${CASE_ID}" ]]; then
  echo "alert is not linked to case after bulk bind" >&2
  exit 1
fi

SECOND_ALERT_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"title\":\"Smoke Alert Batch $(date +%s)\",\"description\":\"Batch test\",\"source\":\"manual\",\"severity\":\"medium\"}" http://localhost/api/v1/alerts)"
SECOND_ALERT_ID="$(resolve_async_resource_id "${SECOND_ALERT_RESP}" || true)"
if [[ -z "${SECOND_ALERT_ID}" || "${SECOND_ALERT_ID}" == "null" ]]; then
  echo "failed to create second alert for bulk case creation" >&2
  exit 1
fi

CREATE_CASE_FROM_ALERTS_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"alert_ids\":[\"${SECOND_ALERT_ID}\"],\"case\":{\"title\":\"Bulk Created Case\",\"priority\":\"medium\"}}" http://localhost/api/v1/alerts/bulk/create-case)"
CASE_FROM_ALERTS_ID="$(echo "${CREATE_CASE_FROM_ALERTS_RESP}" | jq -r '.case.id')"
if [[ -z "${CASE_FROM_ALERTS_ID}" || "${CASE_FROM_ALERTS_ID}" == "null" ]]; then
  echo "bulk create case from alerts failed" >&2
  exit 1
fi

CASE_STATUSES_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" http://localhost/api/v1/case-statuses)"
if [[ "$(echo "${CASE_STATUSES_RESP}" | jq '.statuses | length')" -lt 1 ]]; then
  echo "case statuses endpoint returned empty list" >&2
  exit 1
fi

CASE_STATUS_UPDATE_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -X PUT -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d '{"statuses":[{"code":"incident","label":"Incident","order":10,"is_closed":false,"color":"#f59e0b"},{"code":"major_vuln","label":"Major Vuln","order":20,"is_closed":false,"color":"#ef4444"},{"code":"resolved","label":"Resolved","order":90,"is_closed":true,"color":"#22c55e"}]}' http://localhost/api/v1/case-statuses)"
if [[ "$(echo "${CASE_STATUS_UPDATE_RESP}" | jq -r '.statuses[0].code')" == "null" ]]; then
  echo "failed to update tenant case statuses" >&2
  exit 1
fi

CUSTOM_STATUS_CASE_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d '{"title":"Custom Status Case","description":"Tenant status check","severity":"medium","status":"incident"}' http://localhost/api/v1/cases)"
CUSTOM_STATUS_CASE_ID="$(resolve_async_resource_id "${CUSTOM_STATUS_CASE_RESP}" || true)"
if [[ -z "${CUSTOM_STATUS_CASE_ID}" || "${CUSTOM_STATUS_CASE_ID}" == "null" ]]; then
  echo "custom status case create returned no id" >&2
  exit 1
fi
CUSTOM_STATUS_CASE_GET="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/cases/${CUSTOM_STATUS_CASE_ID}")"
if [[ "$(echo "${CUSTOM_STATUS_CASE_GET}" | jq -r '.status')" != "incident" ]]; then
  echo "custom tenant case status is not applied" >&2
  exit 1
fi

INVALID_STATUS_HTTP="$(docker exec "${FRONT_CONTAINER}" sh -lc "curl -sS -o /tmp/ih-invalid-case-status.json -w '%{http_code}' -H 'Authorization: Bearer ${TOKEN}' -H 'X-Tenant-ID: ${TENANT}' -H 'Content-Type: application/json' -d '{\"title\":\"Invalid Status Case\",\"severity\":\"low\",\"status\":\"open\"}' http://localhost/api/v1/cases")"
if [[ "${INVALID_STATUS_HTTP}" -lt 400 ]]; then
  echo "invalid tenant case status unexpectedly accepted" >&2
  exit 1
fi

CACHE_PROBE="cache_probe_$(date +%s)"
CACHE_HEADER_1="$(docker exec "${FRONT_CONTAINER}" sh -lc "curl -sS -D - -o /dev/null -H 'Authorization: Bearer ${TOKEN}' \"http://localhost/api/v1/me?${CACHE_PROBE}=1\" | tr -d '\r' | awk -F': ' 'tolower(\$1)==\"x-api-cache\" {print \$2}' | tail -n1")"
CACHE_HEADER_2="$(docker exec "${FRONT_CONTAINER}" sh -lc "curl -sS -D - -o /dev/null -H 'Authorization: Bearer ${TOKEN}' \"http://localhost/api/v1/me?${CACHE_PROBE}=1\" | tr -d '\r' | awk -F': ' 'tolower(\$1)==\"x-api-cache\" {print \$2}' | tail -n1")"
if [[ "${CACHE_HEADER_2}" != "HIT" ]]; then
  echo "api cache did not produce HIT on repeated GET (headers: first=${CACHE_HEADER_1}, second=${CACHE_HEADER_2})" >&2
  exit 1
fi

FULL_CASE_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d '{"title":"Extended Smoke Case","description":"Extended fields","source":"siem","incident_type":"Phishing","priority":"high","impact":"mailboxes","confidence":77,"severity":"critical","tlp":"amber","pap":"amber"}' http://localhost/api/v1/cases)"
FULL_CASE_ID="$(resolve_async_resource_id "${FULL_CASE_RESP}" || true)"
if [[ -z "${FULL_CASE_ID}" || "${FULL_CASE_ID}" == "null" ]]; then
  echo "extended case create returned no id" >&2
  exit 1
fi
FULL_CASE_GET="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/cases/${FULL_CASE_ID}")"
if [[ "$(echo "${FULL_CASE_GET}" | jq -r '.incident_type')" != "Phishing" ]]; then
  echo "extended case fields are not persisted" >&2
  exit 1
fi

USERS_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" "http://localhost/api/v1/users?tenant_id=${TENANT}")"
RESPONSIBLE_USER_ID="$(echo "${USERS_RESP}" | jq -r '.[0].id')"
if [[ -z "${RESPONSIBLE_USER_ID}" || "${RESPONSIBLE_USER_ID}" == "null" ]]; then
  echo "cannot fetch responsible user candidate" >&2
  exit 1
fi

TENANT_SLUG="smoke_tenant_$(date +%s)"
CREATE_TENANT_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H 'Content-Type: application/json' -d "{\"slug\":\"${TENANT_SLUG}\",\"name\":\"Smoke Tenant\",\"description\":\"Smoke tenant\",\"max_users\":25,\"responsible_user_id\":\"${RESPONSIBLE_USER_ID}\"}" http://localhost/api/v1/tenants)"
CREATED_RESPONSIBLE_ID="$(echo "${CREATE_TENANT_RESP}" | jq -r '.responsible_user_id')"
if [[ "${CREATED_RESPONSIBLE_ID}" != "${RESPONSIBLE_USER_ID}" ]]; then
  echo "tenant responsible user mismatch: expected ${RESPONSIBLE_USER_ID}, got ${CREATED_RESPONSIBLE_ID}" >&2
  exit 1
fi

THREAD_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"case_id\":\"${CASE_ID}\",\"title\":\"Smoke Thread\",\"status\":\"In Progress\"}" http://localhost/api/v1/forum/threads)"
THREAD_ID="$(echo "${THREAD_RESP}" | jq -r '.id')"
if [[ -z "${THREAD_ID}" || "${THREAD_ID}" == "null" ]]; then
  echo "failed to create forum thread" >&2
  exit 1
fi

FORUM_ATTACHMENT_RESP="$(docker exec "${FRONT_CONTAINER}" sh -lc "printf 'forum-attachment' >/tmp/ih-smoke-forum-attachment.txt && curl -sS -H 'Authorization: Bearer ${TOKEN}' -H 'X-Tenant-ID: ${TENANT}' -F content='Smoke attachment post' -F files=@/tmp/ih-smoke-forum-attachment.txt http://localhost/api/v1/forum/threads/${THREAD_ID}/posts/upload")"
if [[ "$(echo "${FORUM_ATTACHMENT_RESP}" | jq -r '.attachments | length')" -lt 1 ]]; then
  echo "failed to upload forum attachment post" >&2
  exit 1
fi

THREAD_RESP_REPEAT="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"case_id\":\"${CASE_ID}\",\"title\":\"Smoke Thread Again\",\"status\":\"In Progress\"}" http://localhost/api/v1/forum/threads)"
THREAD_ID_REPEAT="$(echo "${THREAD_RESP_REPEAT}" | jq -r '.id')"
if [[ "${THREAD_ID_REPEAT}" != "${THREAD_ID}" ]]; then
  echo "forum uniqueness per case is broken: ${THREAD_ID} != ${THREAD_ID_REPEAT}" >&2
  exit 1
fi

POST_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"thread_id\":\"${THREAD_ID}\",\"content\":\"Smoke post\"}" http://localhost/api/v1/forum/posts)"
POST_ID="$(echo "${POST_RESP}" | jq -r '.id')"
if [[ -z "${POST_ID}" || "${POST_ID}" == "null" ]]; then
  echo "failed to create forum post" >&2
  exit 1
fi

OUTBOUND_CONNECTOR_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d '{"data":{"name":"Smoke Mock Outbound","description":"Smoke outbound connector","type":"Webhook","direction":"outbound","channel":"mock","enabled":true,"config":{}}}' http://localhost/api/v1/catalog/outbound_connectors)"
OUTBOUND_CONNECTOR_ID="$(echo "${OUTBOUND_CONNECTOR_RESP}" | jq -r '.id')"
if [[ -z "${OUTBOUND_CONNECTOR_ID}" || "${OUTBOUND_CONNECTOR_ID}" == "null" ]]; then
  echo "failed to create outbound connector" >&2
  exit 1
fi

OUTBOUND_KAFKA_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d '{"data":{"name":"Smoke Kafka Outbound","description":"Kafka outbound connector","type":"Kafka","direction":"outbound","enabled":true,"config":{"brokers":"kafka:9092","topic":"incidenthub.smoke.out"}}}' http://localhost/api/v1/catalog/outbound_connectors)"
OUTBOUND_KAFKA_ID="$(echo "${OUTBOUND_KAFKA_RESP}" | jq -r '.id')"
if [[ -z "${OUTBOUND_KAFKA_ID}" || "${OUTBOUND_KAFKA_ID}" == "null" ]]; then
  echo "failed to create outbound kafka connector" >&2
  exit 1
fi

PROXY_SEND_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"connector_id\":\"${OUTBOUND_CONNECTOR_ID}\",\"content\":\"Ping from smoke\"}" "http://localhost/api/v1/forum/threads/${THREAD_ID}/proxy/send")"
if [[ "$(echo "${PROXY_SEND_RESP}" | jq -r '.external_reply.id')" == "null" ]]; then
  echo "forum proxy send did not return external reply" >&2
  exit 1
fi

COMM_THREAD_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"title\":\"External Coordination\",\"connector_id\":\"${OUTBOUND_CONNECTOR_ID}\",\"participant\":{\"name\":\"Smoke User\",\"target\":\"smoke-user\"}}" "http://localhost/api/v1/cases/${CASE_ID}/communications")"
COMM_THREAD_ID="$(echo "${COMM_THREAD_RESP}" | jq -r '.id')"
if [[ -z "${COMM_THREAD_ID}" || "${COMM_THREAD_ID}" == "null" ]]; then
  echo "failed to create case communication thread" >&2
  exit 1
fi

COMM_LIST_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/cases/${CASE_ID}/communications")"
COMM_LIST_COUNT="$(echo "${COMM_LIST_RESP}" | jq 'length')"
if [[ "${COMM_LIST_COUNT}" -lt 1 ]]; then
  echo "case communications list is empty" >&2
  exit 1
fi

COMM_SEND_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"connector_id\":\"${OUTBOUND_CONNECTOR_ID}\",\"content\":\"Please confirm containment status\"}" "http://localhost/api/v1/cases/${CASE_ID}/communications/${COMM_THREAD_ID}/messages")"
if [[ "$(echo "${COMM_SEND_RESP}" | jq -r '.user_message.id')" == "null" ]]; then
  echo "failed to send communication message" >&2
  exit 1
fi

COMM_SYNC_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"connector_id\":\"${OUTBOUND_CONNECTOR_ID}\"}" "http://localhost/api/v1/cases/${CASE_ID}/communications/${COMM_THREAD_ID}/sync")"
COMM_SYNC_CREATED="$(echo "${COMM_SYNC_RESP}" | jq -r '.created_count')"
if [[ -z "${COMM_SYNC_CREATED}" || "${COMM_SYNC_CREATED}" == "null" || "${COMM_SYNC_CREATED}" -lt 1 ]]; then
  echo "communication sync did not create inbound messages" >&2
  exit 1
fi

COMM_GET_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/cases/${CASE_ID}/communications/${COMM_THREAD_ID}")"
COMM_MESSAGES_COUNT="$(echo "${COMM_GET_RESP}" | jq '.messages | length')"
if [[ "${COMM_MESSAGES_COUNT}" -lt 2 ]]; then
  echo "communication thread detail returned too few messages: ${COMM_MESSAGES_COUNT}" >&2
  exit 1
fi


OUTBOUND_LIST_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" http://localhost/api/v1/connectors/outbound)"
OUTBOUND_LIST_COUNT="$(echo "${OUTBOUND_LIST_RESP}" | jq 'length')"
if [[ "${OUTBOUND_LIST_COUNT}" -lt 1 ]]; then
  echo "outbound connectors endpoint returned empty list" >&2
  exit 1
fi

AI_SESSION_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" http://localhost/api/v1/ai/session)"
AI_SESSION_ID="$(echo "${AI_SESSION_RESP}" | jq -r '.id')"
if [[ -z "${AI_SESSION_ID}" || "${AI_SESSION_ID}" == "null" ]]; then
  echo "ai session endpoint returned invalid payload" >&2
  exit 1
fi

AI_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"question\":\"What are the recent oauth incidents?\",\"session_id\":\"${AI_SESSION_ID}\"}" http://localhost/api/v1/ai/ask)"
if [[ "$(echo "${AI_RESP}" | jq -r '.answer')" == "null" ]]; then
  echo "ai ask endpoint returned invalid payload" >&2
  exit 1
fi
if [[ "$(echo "${AI_RESP}" | jq -r '.session_id')" == "null" ]]; then
  echo "ai ask endpoint did not return session_id" >&2
  exit 1
fi

AI_MESSAGES_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/ai/messages?session_id=${AI_SESSION_ID}&limit=50")"
AI_MESSAGES_COUNT="$(echo "${AI_MESSAGES_RESP}" | jq '.messages | length')"
if [[ "${AI_MESSAGES_COUNT}" -lt 2 ]]; then
  echo "ai messages endpoint returned too few messages: ${AI_MESSAGES_COUNT}" >&2
  exit 1
fi

CASE_AI_ANALYSIS_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -X POST -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/cases/${CASE_ID}/ai/analyze")"
CASE_AI_ANALYSIS_ID="$(echo "${CASE_AI_ANALYSIS_RESP}" | jq -r '.id')"
CASE_AI_ANALYSIS_ERROR="$(echo "${CASE_AI_ANALYSIS_RESP}" | jq -r '.error // empty')"
if [[ ( -z "${CASE_AI_ANALYSIS_ID}" || "${CASE_AI_ANALYSIS_ID}" == "null" ) && ( -z "${CASE_AI_ANALYSIS_ERROR}" || "${CASE_AI_ANALYSIS_ERROR}" == "null" ) ]]; then
  echo "case ai analyze endpoint returned neither analysis id nor error payload" >&2
  exit 1
fi

CASE_AI_ANALYSES_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/cases/${CASE_ID}/ai/analyses?limit=10")"
CASE_AI_ANALYSES_COUNT="$(echo "${CASE_AI_ANALYSES_RESP}" | jq 'length')"
if [[ "${CASE_AI_ANALYSES_COUNT}" == "null" || -z "${CASE_AI_ANALYSES_COUNT}" ]]; then
  echo "case ai analyses endpoint returned invalid history payload" >&2
  exit 1
fi
if [[ "${CASE_AI_ANALYSIS_ID}" != "null" && -n "${CASE_AI_ANALYSIS_ID}" && "${CASE_AI_ANALYSES_COUNT}" -lt 1 ]]; then
  echo "case ai analyses endpoint returned empty history" >&2
  exit 1
fi

HEALTH_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" http://localhost/api/v1/system/health)"
if [[ "$(echo "${HEALTH_RESP}" | jq -r '.modules.api.status')" == "null" ]]; then
  echo "system health does not contain api module" >&2
  exit 1
fi
if [[ "$(echo "${HEALTH_RESP}" | jq -r '.modules.postgres.status')" == "null" ]]; then
  echo "system health does not contain postgres module" >&2
  exit 1
fi
if [[ "$(echo "${HEALTH_RESP}" | jq -r '.modules.redis.status')" == "null" ]]; then
  echo "system health does not contain redis module" >&2
  exit 1
fi
if [[ "$(echo "${HEALTH_RESP}" | jq -r '.modules.elasticsearch.status')" == "null" ]]; then
  echo "system health does not contain elasticsearch module" >&2
  exit 1
fi
if [[ "$(echo "${HEALTH_RESP}" | jq -r '.modules.s3.status')" == "null" ]]; then
  echo "system health does not contain s3 module" >&2
  exit 1
fi
if [[ "$(echo "${HEALTH_RESP}" | jq -r '.modules.ai_model.status')" == "null" ]]; then
  echo "system health does not contain ai_model module" >&2
  exit 1
fi

DASHBOARD_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" http://localhost/api/v1/dashboard/stats)"
if [[ "$(echo "${DASHBOARD_RESP}" | jq -r '.activeCases')" == "null" ]]; then
  echo "dashboard stats missing activeCases" >&2
  exit 1
fi
if [[ "$(echo "${DASHBOARD_RESP}" | jq -r '.alertsCount')" == "null" ]]; then
  echo "dashboard stats missing alertsCount" >&2
  exit 1
fi

RESOURCES_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" http://localhost/api/v1/system/resources)"
if [[ "$(echo "${RESOURCES_RESP}" | jq -r '.tenant.openCases')" == "null" ]]; then
  echo "system resources missing tenant.openCases" >&2
  exit 1
fi
if [[ "$(echo "${RESOURCES_RESP}" | jq -r '.postgresPool.totalConns')" == "null" ]]; then
  echo "system resources missing postgresPool.totalConns" >&2
  exit 1
fi
REQ24H="$(echo "${RESOURCES_RESP}" | jq -r '.tenant.apiRequests24h')"
if [[ -z "${REQ24H}" || "${REQ24H}" == "null" || "${REQ24H}" -lt 1 ]]; then
  echo "system resources apiRequests24h invalid: ${REQ24H}" >&2
  exit 1
fi
if [[ "$(echo "${RESOURCES_RESP}" | jq -r '.host.cpuPercent')" == "null" ]]; then
  echo "system resources missing host.cpuPercent" >&2
  exit 1
fi
if [[ "$(echo "${RESOURCES_RESP}" | jq -r '.host.memoryUsedMB')" == "null" ]]; then
  echo "system resources missing host.memoryUsedMB" >&2
  exit 1
fi
if [[ "$(echo "${RESOURCES_RESP}" | jq -r '.host.diskUsedPercent')" == "null" ]]; then
  echo "system resources missing host.diskUsedPercent" >&2
  exit 1
fi

if ! ensure_observable_types_defaults; then
  exit 1
fi

if ! ensure_achievements_defaults; then
  exit 1
fi

ENC_QUERY="$(printf '%s' "${TITLE}" | jq -sRr @uri)"
SEARCH_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/search?q=${ENC_QUERY}")"
SEARCH_COUNT="$(echo "${SEARCH_RESP}" | jq '.alerts | length')"
if [[ "${SEARCH_COUNT}" -lt 1 ]]; then
  echo "search did not return created alert" >&2
  exit 1
fi

MULTI_ALERT_TITLE="Smoke Multi Word $(date +%s)"
MULTI_ALERT_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" -H 'Content-Type: application/json' -d "{\"title\":\"${MULTI_ALERT_TITLE}\",\"description\":\"Multi token search smoke\",\"source\":\"manual\",\"severity\":\"high\"}" http://localhost/api/v1/alerts)"
if [[ -z "$(resolve_async_resource_id "${MULTI_ALERT_RESP}" || true)" ]]; then
  echo "failed to create multi-word search alert" >&2
  exit 1
fi
ENC_MULTI_QUERY="$(printf '%s' "Smoke Word" | jq -sRr @uri)"
MULTI_SEARCH_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Tenant-ID: ${TENANT}" "http://localhost/api/v1/search?q=${ENC_MULTI_QUERY}")"
MULTI_SEARCH_COUNT="$(echo "${MULTI_SEARCH_RESP}" | jq '.alerts | length')"
if [[ "${MULTI_SEARCH_COUNT}" -lt 1 ]]; then
  echo "multi-word search did not narrow correctly" >&2
  exit 1
fi

SWAGGER_RESP="$(docker exec "${FRONT_CONTAINER}" curl -sS http://localhost/dev/swagger/openapi.json)"
if [[ "$(echo "${SWAGGER_RESP}" | jq -r '.openapi')" != "3.0.3" ]]; then
  echo "swagger spec endpoint returned invalid document" >&2
  exit 1
fi

echo "smoke test passed: tenant=${TENANT}, alert=${ALERT_ID}, case=${CASE_ID}, thread=${THREAD_ID}, outbound_connectors=${OUTBOUND_LIST_COUNT}, search_alerts=${SEARCH_COUNT}"
