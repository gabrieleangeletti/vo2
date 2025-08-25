-- Create extension uuid-ossp.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create extension postgis in its own schema.
CREATE SCHEMA postgis;

GRANT USAGE ON SCHEMA postgis TO public;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA postgis TO public;

GRANT USAGE ON SCHEMA postgis TO postgres;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA postgis TO postgres;

CREATE EXTENSION IF NOT EXISTS postgis WITH SCHEMA postgis;
