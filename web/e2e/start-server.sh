#!/bin/sh
# Starts the API against a fresh, throwaway database for the e2e run.
set -e
cd "$(dirname "$0")/../../server"

docker compose up -d --wait postgres
docker compose exec -T postgres psql -U "${DB_USER:-postgres}" -d postgres \
	-c 'DROP DATABASE IF EXISTS tabulation_e2e WITH (FORCE)' \
	-c 'CREATE DATABASE tabulation_e2e'

go build -o bin/e2e-server .
exec bin/e2e-server
