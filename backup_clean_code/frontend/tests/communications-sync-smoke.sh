#!/usr/bin/env bash
set -euo pipefail

MODE="api"
if [[ "${1:-}" == "--prepare-ui" ]]; then
  MODE="prepare-ui"
  shift
fi

FRONT_CONTAINER="${FRONT_CONTAINER:-incidenthub-frontend}"
MOCK_PORT="${COMM_MOCK_PORT:-18081}"
MOCK_HOST_BIND="${COMM_MOCK_HOST_BIND:-127.0.0.1}"
MOCK_SERVER_BASE="${COMM_MOCK_SERVER_BASE:-http://host.docker.internal:${MOCK_PORT}}"
START_MOCK_SERVER="${START_MOCK_SERVER:-1}"

if ! docker compose ps frontend --status running --format '{{.Name}}' | grep -q "^${FRONT_CONTAINER}$"; then
  echo "frontend container '${FRONT_CONTAINER}' is not running" >&2
  exit 1
fi

MOCK_PID=""
cleanup() {
  if [[ -n "${MOCK_PID}" ]] && kill -0 "${MOCK_PID}" >/dev/null 2>&1; then
    kill "${MOCK_PID}" >/dev/null 2>&1 || true
    wait "${MOCK_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [[ "${START_MOCK_SERVER}" == "1" ]]; then
  python3 frontend/tests/communications_mock_server.py --host "${MOCK_HOST_BIND}" --port "${MOCK_PORT}" >/tmp/incidenthub-communications-mock.log 2>&1 &
  MOCK_PID="$!"
  for _ in $(seq 1 30); do
    if curl -sS "http://${MOCK_HOST_BIND}:${MOCK_PORT}/healthz" >/dev/null 2>&1; then
      break
    fi
    sleep 0.2
  done
  if ! curl -sS "http://${MOCK_HOST_BIND}:${MOCK_PORT}/healthz" >/dev/null 2>&1; then
    echo "communications mock server failed to start; see /tmp/incidenthub-communications-mock.log" >&2
    exit 1
  fi
fi

front_post_anon() {
  local path="$1"
  local payload="$2"
  docker exec -i "${FRONT_CONTAINER}" sh -lc "curl -sS -H 'Content-Type: application/json' --data @/dev/stdin 'http://localhost${path}'" <<<"${payload}"
}

front_get() {
  local path="$1"
  docker exec "${FRONT_CONTAINER}" sh -lc "curl -sS -H 'Authorization: Bearer ${TOKEN}' -H 'X-Tenant-ID: ${TENANT_ID}' 'http://localhost${path}'"
}

front_post() {
  local path="$1"
  local payload="$2"
  docker exec -i "${FRONT_CONTAINER}" sh -lc "curl -sS -H 'Authorization: Bearer ${TOKEN}' -H 'X-Tenant-ID: ${TENANT_ID}' -H 'Content-Type: application/json' --data @/dev/stdin 'http://localhost${path}'" <<<"${payload}"
}

wait_async_operation() {
  local operation_id="$1"
  local attempts="${2:-90}"
  local resp=""
  local status=""
  for _ in $(seq 1 "${attempts}"); do
    resp="$(front_get "/api/v1/operations/${operation_id}")"
    status="$(echo "${resp}" | jq -r '.status // empty')"
    if [[ "${status}" == "done" ]]; then
      echo "${resp}"
      return 0
    fi
    if [[ "${status}" == "failed" ]]; then
      echo "async operation failed: ${resp}" >&2
      return 1
    fi
    sleep 1
  done
  echo "timed out waiting for async operation ${operation_id}" >&2
  return 1
}

resolve_async_resource_id() {
  local create_resp="$1"
  local direct_id=""
  local resource_id=""
  local status=""
  local operation_id=""
  local operation_resp=""

  direct_id="$(echo "${create_resp}" | jq -r '.id // empty')"
  if [[ -n "${direct_id}" && "${direct_id}" != "null" ]]; then
    echo "${direct_id}"
    return 0
  fi

  resource_id="$(echo "${create_resp}" | jq -r '.resource_id // empty')"
  status="$(echo "${create_resp}" | jq -r '.status // empty')"
  operation_id="$(echo "${create_resp}" | jq -r '.operation_id // empty')"
  if [[ "${status}" == "queued" && -n "${operation_id}" && "${operation_id}" != "null" ]]; then
    operation_resp="$(wait_async_operation "${operation_id}")" || return 1
    resource_id="$(echo "${operation_resp}" | jq -r '.resource_id // empty')"
  fi

  if [[ -z "${resource_id}" || "${resource_id}" == "null" ]]; then
    return 1
  fi
  echo "${resource_id}"
}

LOGIN_PAYLOAD='{"email":"admin@incidenthub.local","password":"ChangeMeNow123!"}'
LOGIN_RESP="$(front_post_anon /api/v1/auth/login "${LOGIN_PAYLOAD}")"
TOKEN="$(echo "${LOGIN_RESP}" | jq -r '.access_token // empty')"
TENANT_ID="$(echo "${LOGIN_RESP}" | jq -r '.memberships[0].tenant_id // empty')"
USER_ID="$(echo "${LOGIN_RESP}" | jq -r '.identity.user_id // empty')"
if [[ -z "${TOKEN}" || -z "${TENANT_ID}" || -z "${USER_ID}" || "${TOKEN}" == "null" || "${TENANT_ID}" == "null" ]]; then
  echo "login failed: ${LOGIN_RESP}" >&2
  exit 1
fi

TENANTS_RESP="$(front_get /api/v1/tenants)"
TENANT_SLUG="$(echo "${TENANTS_RESP}" | jq -r --arg tenant_id "${TENANT_ID}" 'map(select(.id == $tenant_id)) | .[0].slug // empty')"
if [[ -z "${TENANT_SLUG}" ]]; then
  TENANT_SLUG="admin"
fi

RUN_SUFFIX="$(date +%s)"
SLACK_CONNECTOR_PAYLOAD="$(jq -nc --arg suffix "${RUN_SUFFIX}" --arg base_url "${MOCK_SERVER_BASE}/slack" '{data:{name:("Smoke Slack Connector " + $suffix),description:"Slack communication smoke connector",type:"Slack",channel:"slack",direction:"outbound",enabled:true,category:"notifications",communication_mode:"chat",capabilities:["hub_execute","case_communications","forum_threads","sync_messages"],config:{botToken:"smoke-token",apiBaseURL:$base_url,channel_id:"C123456789"}}}')"
SLACK_CONNECTOR_RESP="$(front_post /api/v1/catalog/outbound_connectors "${SLACK_CONNECTOR_PAYLOAD}")"
SLACK_CONNECTOR_ID="$(echo "${SLACK_CONNECTOR_RESP}" | jq -r '.id // empty')"
if [[ -z "${SLACK_CONNECTOR_ID}" ]]; then
  echo "failed to create slack connector: ${SLACK_CONNECTOR_RESP}" >&2
  exit 1
fi

OUTLOOK_CONNECTOR_PAYLOAD="$(jq -nc --arg suffix "${RUN_SUFFIX}" --arg api_base_url "${MOCK_SERVER_BASE}/outlook/v1.0" --arg auth_base_url "${MOCK_SERVER_BASE}/outlook-auth" '{data:{name:("Smoke Outlook Connector " + $suffix),description:"Outlook communication smoke connector",type:"Outlook",channel:"outlook",direction:"outbound",enabled:true,category:"notifications",communication_mode:"email",capabilities:["hub_execute","case_communications","forum_threads","sync_messages"],config:{tenantId:"smoke-tenant",clientId:"smoke-client",clientSecret:"smoke-secret",mailbox:"soc@example.com",apiBaseURL:$api_base_url,authBaseURL:$auth_base_url}}}')"
OUTLOOK_CONNECTOR_RESP="$(front_post /api/v1/catalog/outbound_connectors "${OUTLOOK_CONNECTOR_PAYLOAD}")"
OUTLOOK_CONNECTOR_ID="$(echo "${OUTLOOK_CONNECTOR_RESP}" | jq -r '.id // empty')"
if [[ -z "${OUTLOOK_CONNECTOR_ID}" ]]; then
  echo "failed to create outlook connector: ${OUTLOOK_CONNECTOR_RESP}" >&2
  exit 1
fi

CASE_TITLE="Communications Smoke ${RUN_SUFFIX}"
CASE_PAYLOAD="$(jq -nc --arg title "${CASE_TITLE}" '{title:$title,description:"Communications smoke case",severity:"high"}')"
CASE_RESP="$(front_post /api/v1/cases "${CASE_PAYLOAD}")"
CASE_ID="$(resolve_async_resource_id "${CASE_RESP}")"
if [[ -z "${CASE_ID}" ]]; then
  echo "failed to create case: ${CASE_RESP}" >&2
  exit 1
fi

FORUM_PAYLOAD="$(jq -nc --arg case_id "${CASE_ID}" --arg suffix "${RUN_SUFFIX}" '{case_id:$case_id,title:("Forum smoke thread " + $suffix),status:"In Progress",initial_message:"Initial forum thread message"}')"
FORUM_RESP="$(front_post /api/v1/forum/threads "${FORUM_PAYLOAD}")"
FORUM_THREAD_ID="$(resolve_async_resource_id "${FORUM_RESP}")"
if [[ -z "${FORUM_THREAD_ID}" ]]; then
  echo "failed to create forum thread: ${FORUM_RESP}" >&2
  exit 1
fi

COMM_THREAD_PAYLOAD="$(jq -nc --arg title "Outlook coordination" --arg connector_id "${OUTLOOK_CONNECTOR_ID}" --arg subject "Smoke Outlook Subject ${RUN_SUFFIX}" '{title:$title,connector_id:$connector_id,subject:$subject,participant:{name:"External Mailbox",target:"recipient@example.com",email:"recipient@example.com"},metadata:{subject:$subject,to:"recipient@example.com",recipient:"recipient@example.com"}}')"
COMM_THREAD_RESP="$(front_post "/api/v1/cases/${CASE_ID}/communications" "${COMM_THREAD_PAYLOAD}")"
COMM_THREAD_ID="$(resolve_async_resource_id "${COMM_THREAD_RESP}")"
if [[ -z "${COMM_THREAD_ID}" ]]; then
  echo "failed to create communication thread: ${COMM_THREAD_RESP}" >&2
  exit 1
fi

CASE_URL="http://localhost:5173/${TENANT_SLUG}/cases/${CASE_ID}?tab=communications"
FORUM_URL="http://localhost:5173/${TENANT_SLUG}/forum/${FORUM_THREAD_ID}"
CONNECTORS_URL="http://localhost:5173/${TENANT_SLUG}/connectors"

if [[ "${MODE}" == "prepare-ui" ]]; then
  jq -nc \
    --arg tenant_id "${TENANT_ID}" \
    --arg tenant_slug "${TENANT_SLUG}" \
    --arg case_id "${CASE_ID}" \
    --arg communication_thread_id "${COMM_THREAD_ID}" \
    --arg forum_thread_id "${FORUM_THREAD_ID}" \
    --arg slack_connector_id "${SLACK_CONNECTOR_ID}" \
    --arg outlook_connector_id "${OUTLOOK_CONNECTOR_ID}" \
    --arg connectors_url "${CONNECTORS_URL}" \
    --arg case_url "${CASE_URL}" \
    --arg forum_url "${FORUM_URL}" \
    '{tenant_id:$tenant_id,tenant_slug:$tenant_slug,case_id:$case_id,communication_thread_id:$communication_thread_id,forum_thread_id:$forum_thread_id,slack_connector_id:$slack_connector_id,outlook_connector_id:$outlook_connector_id,connectors_url:$connectors_url,case_url:$case_url,forum_url:$forum_url}'
  exit 0
fi

COMM_SEND_PAYLOAD="$(jq -nc --arg connector_id "${OUTLOOK_CONNECTOR_ID}" '{connector_id:$connector_id,content:"Please review the mailbox and confirm containment."}')"
COMM_SEND_RESP="$(front_post "/api/v1/cases/${CASE_ID}/communications/${COMM_THREAD_ID}/messages" "${COMM_SEND_PAYLOAD}")"
if [[ -z "$(echo "${COMM_SEND_RESP}" | jq -r '.user_message.id // empty')" ]]; then
  echo "failed to send communication message: ${COMM_SEND_RESP}" >&2
  exit 1
fi

COMM_SYNC_PAYLOAD="$(jq -nc --arg connector_id "${OUTLOOK_CONNECTOR_ID}" --arg subject "Smoke Outlook Subject ${RUN_SUFFIX}" '{connector_id:$connector_id,subject:$subject}')"
COMM_SYNC_RESP="$(front_post "/api/v1/cases/${CASE_ID}/communications/${COMM_THREAD_ID}/sync" "${COMM_SYNC_PAYLOAD}")"
COMM_SYNC_COUNT="$(echo "${COMM_SYNC_RESP}" | jq -r '.created_count // 0')"
COMM_SYNC_AT="$(echo "${COMM_SYNC_RESP}" | jq -r '.synced_at // empty')"
if [[ "${COMM_SYNC_COUNT}" -lt 1 || -z "${COMM_SYNC_AT}" ]]; then
  echo "communication sync did not return expected metadata: ${COMM_SYNC_RESP}" >&2
  exit 1
fi

COMM_THREAD_GET="$(front_get "/api/v1/cases/${CASE_ID}/communications/${COMM_THREAD_ID}")"
if [[ "$(echo "${COMM_THREAD_GET}" | jq -r '.last_sync_created_count // 0')" -lt 1 ]]; then
  echo "communication thread missing last_sync_created_count: ${COMM_THREAD_GET}" >&2
  exit 1
fi
if [[ -z "$(echo "${COMM_THREAD_GET}" | jq -r '.last_synced_at // empty')" ]]; then
  echo "communication thread missing last_synced_at: ${COMM_THREAD_GET}" >&2
  exit 1
fi
if ! echo "${COMM_THREAD_GET}" | jq -e '.messages[]? | select((.content // "") | contains("Outlook reply:"))' >/dev/null; then
  echo "outlook reply was not imported into communication thread: ${COMM_THREAD_GET}" >&2
  exit 1
fi

FORUM_SEND_PAYLOAD="$(jq -nc --arg connector_id "${SLACK_CONNECTOR_ID}" '{connector_id:$connector_id,binding_key:"C123456789",content:"Please mirror this update into Slack.",author:"Smoke Analyst",metadata:{channel_id:"C123456789"}}')"
FORUM_SEND_RESP="$(front_post "/api/v1/forum/threads/${FORUM_THREAD_ID}/proxy/send" "${FORUM_SEND_PAYLOAD}")"
FORUM_PROFILE_ID="$(echo "${FORUM_SEND_RESP}" | jq -r '.profile_id // empty')"
FORUM_BINDING_KEY="$(echo "${FORUM_SEND_RESP}" | jq -r '.binding_key // empty')"
if [[ -z "${FORUM_PROFILE_ID}" || -z "${FORUM_BINDING_KEY}" ]]; then
  echo "forum proxy send did not create persisted profile metadata: ${FORUM_SEND_RESP}" >&2
  exit 1
fi

FORUM_SYNC_PAYLOAD="$(jq -nc --arg profile_id "${FORUM_PROFILE_ID}" '{profile_id:$profile_id}')"
FORUM_SYNC_RESP="$(front_post "/api/v1/forum/threads/${FORUM_THREAD_ID}/proxy/sync" "${FORUM_SYNC_PAYLOAD}")"
FORUM_SYNC_COUNT="$(echo "${FORUM_SYNC_RESP}" | jq -r '.created_count // 0')"
FORUM_SYNC_AT="$(echo "${FORUM_SYNC_RESP}" | jq -r '.synced_at // empty')"
if [[ "${FORUM_SYNC_COUNT}" -lt 1 || -z "${FORUM_SYNC_AT}" ]]; then
  echo "forum sync did not return expected metadata: ${FORUM_SYNC_RESP}" >&2
  exit 1
fi

FORUM_THREAD_GET="$(front_get "/api/v1/forum/threads/${FORUM_THREAD_ID}")"
if ! echo "${FORUM_THREAD_GET}" | jq -e '.proxy_profiles[]? | select(.id == $profile_id and .has_binding == true and (.last_synced_at // "") != "")' --arg profile_id "${FORUM_PROFILE_ID}" >/dev/null; then
  echo "forum thread payload missing enriched proxy profile sync state: ${FORUM_THREAD_GET}" >&2
  exit 1
fi
if ! echo "${FORUM_THREAD_GET}" | jq -e '.posts[]? | select((.content // "") | contains("Slack reply:"))' >/dev/null; then
  echo "forum thread missing imported slack reply: ${FORUM_THREAD_GET}" >&2
  exit 1
fi

jq -nc \
  --arg tenant_id "${TENANT_ID}" \
  --arg tenant_slug "${TENANT_SLUG}" \
  --arg case_id "${CASE_ID}" \
  --arg communication_thread_id "${COMM_THREAD_ID}" \
  --arg forum_thread_id "${FORUM_THREAD_ID}" \
  --arg slack_connector_id "${SLACK_CONNECTOR_ID}" \
  --arg outlook_connector_id "${OUTLOOK_CONNECTOR_ID}" \
  --arg forum_profile_id "${FORUM_PROFILE_ID}" \
  --arg connectors_url "${CONNECTORS_URL}" \
  --arg case_url "${CASE_URL}" \
  --arg forum_url "${FORUM_URL}" \
  --argjson communication_created_count "${COMM_SYNC_COUNT}" \
  --argjson forum_created_count "${FORUM_SYNC_COUNT}" \
  '{tenant_id:$tenant_id,tenant_slug:$tenant_slug,case_id:$case_id,communication_thread_id:$communication_thread_id,forum_thread_id:$forum_thread_id,slack_connector_id:$slack_connector_id,outlook_connector_id:$outlook_connector_id,forum_profile_id:$forum_profile_id,communication_created_count:$communication_created_count,forum_created_count:$forum_created_count,connectors_url:$connectors_url,case_url:$case_url,forum_url:$forum_url}'
