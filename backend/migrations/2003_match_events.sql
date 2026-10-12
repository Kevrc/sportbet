-- Persona 2: sucesos del partido en vivo (goles, tarjetas, cambios, VAR).
-- API-Football los entrega gratis en fixtures?live=all; se guarda la lista
-- completa del último dato recibido.

-- +goose Up
CREATE TABLE match_events (
    event_id uuid NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    seq      INTEGER NOT NULL,
    minute   INTEGER NOT NULL CHECK (minute >= 0),
    extra    INTEGER NOT NULL DEFAULT 0 CHECK (extra >= 0),
    side     TEXT NOT NULL CHECK (side IN ('HOME', 'AWAY')),
    type     TEXT NOT NULL CHECK (type IN ('GOAL', 'CARD', 'SUBST', 'VAR')),
    detail   TEXT NOT NULL DEFAULT '',
    player   TEXT NOT NULL DEFAULT '',
    assist   TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (event_id, seq)
);

-- +goose Down
DROP TABLE match_events;
