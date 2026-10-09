#!/bin/bash
# Creates one database and one least-privilege role per service.
# Runs once, on first initialisation of the postgres volume.
set -euo pipefail

create_service_db() {
  local name="$1" password="$2"
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<SQL
CREATE ROLE ${name} LOGIN PASSWORD '${password}';
CREATE DATABASE ${name} OWNER ${name};
REVOKE ALL ON DATABASE ${name} FROM PUBLIC;
SQL
}

create_service_db identity  "${IDENTITY_DB_PASSWORD}"
create_service_db catalog   "${CATALOG_DB_PASSWORD}"
create_service_db inventory "${INVENTORY_DB_PASSWORD}"
create_service_db orders    "${ORDER_DB_PASSWORD}"
create_service_db payment   "${PAYMENT_DB_PASSWORD}"
