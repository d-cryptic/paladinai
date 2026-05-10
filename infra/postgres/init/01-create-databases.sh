#!/bin/bash
# Create multiple databases on init (for Hatchet + Paladin)
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    SELECT 'CREATE DATABASE hatchet' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'hatchet')\gexec
    GRANT ALL PRIVILEGES ON DATABASE hatchet TO $POSTGRES_USER;
EOSQL
