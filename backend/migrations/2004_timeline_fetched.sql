-- Persona 2: marca cuándo se pidió la línea de tiempo completa de un partido
-- terminado (fixtures?id trae los sucesos; fixtures?date no). Se pide una sola
-- vez, aunque el partido no haya tenido sucesos.

-- +goose Up
ALTER TABLE events ADD COLUMN timeline_fetched_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE events DROP COLUMN timeline_fetched_at;
