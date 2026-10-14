-- Runs once, when the data volume is first initialised. Creates the login role the
-- services use. It belongs to pharmachat_app, so it is subject to row-level security;
-- migrations grant table privileges to pharmachat_app.
-- Changing APP_DB_PASSWORD later needs: ALTER ROLE pharmachat_api PASSWORD '...';
\set ON_ERROR_STOP on
\getenv app_password APP_DB_PASSWORD
CREATE ROLE pharmachat_app NOLOGIN;
CREATE ROLE pharmachat_api LOGIN PASSWORD :'app_password' IN ROLE pharmachat_app;
