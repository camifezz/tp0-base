#!/usr/bin/env bash

if [ "$#" -ne 2 ]; then
  echo "Uso: $0 <archivo-salida> <cantidad-clientes>" >&2
  exit 1
fi

output_file="$1"
client_count="$2"

if ! [[ "$client_count" =~ ^[0-9]+$ ]]; then
  echo "La cantidad de clientes debe ser un entero mayor o igual a cero" >&2
  exit 1
fi

cat > "$output_file" <<'COMPOSE'
name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
    networks:
      - testing_net
    volumes:
      - ./server/config.ini:/config.ini:ro
COMPOSE

if [ "$client_count" -gt 0 ]; then
  for i in $(seq 1 "$client_count"); do
    cat >> "$output_file" <<COMPOSE

  client${i}:
    container_name: client${i}
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID=${i}
    volumes:
      - ./client/config.yaml:/config.yaml:ro
      - ./.data/dataset/agency-${i}.csv:/agency-${i}.csv:ro
    networks:
      - testing_net
    depends_on:
      - server
COMPOSE
  done
fi

cat >> "$output_file" <<'COMPOSE'

networks:
  testing_net:
    ipam:
      driver: default
      config:
        - subnet: 172.25.125.0/24
COMPOSE

echo "Archivo generado: ${output_file} con ${client_count} clientes"
