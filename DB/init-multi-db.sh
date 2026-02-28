#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE pos_tenant1;
    CREATE DATABASE pos_tenant2;
    CREATE DATABASE pos_tenant3;
EOSQL
