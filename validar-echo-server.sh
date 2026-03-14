#!/usr/bin/env bash

SERVER_SERVICE="server"
SERVER_PORT=12345
MESSAGE="tst server"

fail() {
  echo "action: test_echo_server | result: fail"
  exit 1
}

command -v docker >/dev/null 2>&1 || fail

server_container=$(docker ps -q -f name=^/server$)
[ -n "$server_container" ] || fail

network=$(docker inspect -f '{{range $name,$settings := .NetworkSettings.Networks}}{{printf "%s\n" $name}}{{end}}' "$server_container" | head -n1)
[ -n "$network" ] || fail

response=$(docker run --rm --network "$network" busybox:1.36 sh -c "printf '%s\n' '$MESSAGE' | nc -w 3 ${SERVER_SERVICE} ${SERVER_PORT}" 2>/dev/null || true)

response=$(printf "%s" "$response" | tr -d '\r' | sed 's/[[:space:]]*$//')

if [ "$response" = "$MESSAGE" ]; then
  echo "action: test_echo_server | result: success"
else
  fail
fi
