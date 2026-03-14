#!/usr/bin/env bash
set -euo pipefail

SERVER_SERVICE="server"
MESSAGE="echo-test::$(date +%s%N)"

fail() {
  echo "action: test_echo_server | result: fail"
  exit 1
}

if ! command -v docker >/dev/null 2>&1; then
  fail
fi

server_port=$(awk -F '=' '/SERVER_PORT/{gsub(/ /, "", $2); print $2; exit}' server/config.ini 2>/dev/null || true)
server_port=${server_port:-12345}

server_container=$(docker ps --filter "name=^/${SERVER_SERVICE}$" --format "{{.ID}}" | head -n 1)
[ -n "$server_container" ] || fail

network=$(docker inspect -f '{{range $name,$settings := .NetworkSettings.Networks}}{{printf "%s\n" $name}}{{end}}' "$server_container" 2>/dev/null | head -n1)
[ -n "$network" ] || fail

response=""
if ! response=$(docker run --rm --network "$network" busybox:1.36 sh -c "printf '%s' \"$MESSAGE\" | nc -w 3 ${SERVER_SERVICE} ${server_port}" 2>/dev/null); then
  fail
fi

if [ "$response" = "$MESSAGE" ]; then
  echo "action: test_echo_server | result: success"
else
  fail
fi
