-- Datos para la prueba de carga: una liga y tres partidos con inicio en
-- 3 horas. El worker de markets les crea los mercados en ~15 s.
-- Uso: psql "$DATABASE_URL" -f loadtest/seed.sql   (base aparte, no la de la demo)
WITH l AS (INSERT INTO leagues (name, country) VALUES ('Liga Carga', 'Pruebas') RETURNING id),
     t AS (INSERT INTO teams (name, strength) VALUES
             ('Carga Local A', 1.20), ('Carga Visita A', 0.90),
             ('Carga Local B', 1.00), ('Carga Visita B', 1.00),
             ('Carga Local C', 0.80), ('Carga Visita C', 1.30)
           RETURNING id, name)
INSERT INTO events (league_id, home_team_id, away_team_id, start_at, featured)
SELECT l.id, h.id, a.id, now() + interval '3 hours', true
FROM l,
     (VALUES ('Carga Local A', 'Carga Visita A'),
             ('Carga Local B', 'Carga Visita B'),
             ('Carga Local C', 'Carga Visita C')) AS p(home, away)
JOIN t h ON h.name = p.home
JOIN t a ON a.name = p.away;
