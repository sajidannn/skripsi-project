#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE pos_tenant1;
    CREATE DATABASE pos_tenant2;
    CREATE DATABASE pos_tenant3;
    CREATE DATABASE pos_tenant4;
    CREATE DATABASE pos_tenant5;
    CREATE DATABASE pos_tenant6;
    CREATE DATABASE pos_tenant7;
    CREATE DATABASE pos_tenant8;
    CREATE DATABASE pos_tenant9;
    CREATE DATABASE pos_tenant10;
    CREATE DATABASE pos_tenant11;
    CREATE DATABASE pos_tenant12;
    CREATE DATABASE pos_tenant13;
    CREATE DATABASE pos_tenant14;
    CREATE DATABASE pos_tenant15;
    CREATE DATABASE pos_tenant16;
    CREATE DATABASE pos_tenant17;
    CREATE DATABASE pos_tenant18;
    CREATE DATABASE pos_tenant19;
    CREATE DATABASE pos_tenant20;
EOSQL
