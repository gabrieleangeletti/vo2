-- Create extension postgis in its own schema.
CREATE SCHEMA postgis;

GRANT USAGE ON SCHEMA postgis TO public;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA postgis TO public;

GRANT USAGE ON SCHEMA postgis TO vo2;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA postgis TO vo2;

CREATE EXTENSION IF NOT EXISTS postgis WITH SCHEMA postgis;
