-- Persona 2: escudo de cada equipo (URL https que entrega API-Football).

-- +goose Up
ALTER TABLE teams ADD COLUMN logo_url TEXT;

-- +goose Down
ALTER TABLE teams DROP COLUMN logo_url;
