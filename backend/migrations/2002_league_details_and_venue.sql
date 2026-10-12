-- Persona 2: datos que API-Football ya entrega en cada partido.

-- +goose Up
ALTER TABLE leagues
    ADD COLUMN season   INTEGER,
    ADD COLUMN logo_url TEXT,
    ADD COLUMN flag_url TEXT;
ALTER TABLE events
    ADD COLUMN round      TEXT,
    ADD COLUMN venue_name TEXT,
    ADD COLUMN venue_city TEXT;

-- +goose Down
ALTER TABLE events DROP COLUMN round, DROP COLUMN venue_name, DROP COLUMN venue_city;
ALTER TABLE leagues DROP COLUMN season, DROP COLUMN logo_url, DROP COLUMN flag_url;
