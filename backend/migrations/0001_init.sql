-- Migración base de SportBet. CONGELADA: nadie la edita.
-- Cambios nuevos van en migraciones con prefijo por persona (1xxx_, 2xxx_, 3xxx_).
--
-- Convenciones:
--   * IDs uuid con gen_random_uuid().
--   * Dinero en BIGINT de centavos (*_cents). Nunca float.
--   * Cuotas en INTEGER de centésimas (odd_x100: 214 = 2.14), mínimo 101.
--   * Estados como TEXT con CHECK, no ENUM, para que ampliarlos sea fácil.
--   * Fechas TIMESTAMPTZ en UTC.
--   * Cada tabla la lee y escribe solo su módulo dueño (indicado en cada bloque).

-- +goose Up

-- ===== auth / users (Persona 1) =====
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  BYTEA NOT NULL UNIQUE, -- sha256 del token opaco
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

-- ===== admin (Persona 1) =====
CREATE TABLE audit_log (
    id         BIGSERIAL PRIMARY KEY,
    actor_id   uuid REFERENCES users (id),
    action     TEXT NOT NULL,
    entity     TEXT NOT NULL,
    entity_id  TEXT,
    data       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC);

-- ===== events (Persona 2) =====
CREATE TABLE leagues (
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name     TEXT NOT NULL,
    country  TEXT NOT NULL DEFAULT '',
    featured BOOLEAN NOT NULL DEFAULT false
);

-- La relación equipo–liga sale de los eventos: teams no tiene league_id.
CREATE TABLE teams (
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name     TEXT NOT NULL,
    -- Fuerza para el modelo de cuotas (0.60 – 1.60, 1.00 = promedio).
    strength NUMERIC(4, 2) NOT NULL DEFAULT 1.00 CHECK (strength BETWEEN 0.60 AND 1.60)
);

CREATE TABLE events (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id       uuid NOT NULL REFERENCES leagues (id),
    home_team_id    uuid NOT NULL REFERENCES teams (id),
    away_team_id    uuid NOT NULL REFERENCES teams (id),
    start_at        TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN (
                        'SCHEDULED', 'LIVE', 'INTERRUPTED', 'FINISHED',
                        'POSTPONED', 'CANCELLED', 'ABANDONED', 'AWARDED')),
    -- Marcador de tiempo reglamentario (score.fulltime), sin prórroga ni penales.
    home_score      INTEGER,
    away_score      INTEGER,
    minute          INTEGER,
    featured        BOOLEAN NOT NULL DEFAULT false,
    kicked_off_at   TIMESTAMPTZ, -- inicio real observado
    live_updated_at TIMESTAMPTZ, -- el frontend marca desactualizado si > 5 min
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (home_team_id <> away_team_id)
);
CREATE INDEX events_start_idx ON events (start_at);
CREATE INDEX events_status_idx ON events (status);

-- Mapea IDs del proveedor a IDs internos.
CREATE TABLE external_refs (
    entity_type TEXT NOT NULL CHECK (entity_type IN ('league', 'team', 'event')),
    entity_id   uuid NOT NULL,
    provider    TEXT NOT NULL,
    external_id TEXT NOT NULL,
    PRIMARY KEY (provider, entity_type, external_id),
    UNIQUE (entity_type, entity_id, provider)
);

-- ===== markets (Persona 2) =====
CREATE TABLE markets (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   uuid NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    code       TEXT NOT NULL CHECK (code IN ('MATCH_RESULT', 'TOTAL_GOALS_2_5', 'BOTH_TEAMS_TO_SCORE')),
    status     TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED', 'SETTLED', 'VOID')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, code)
);

-- La cuota no es entidad: es la columna selections.odd_x100.
CREATE TABLE selections (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    market_id uuid NOT NULL REFERENCES markets (id) ON DELETE CASCADE,
    code      TEXT NOT NULL CHECK (code IN ('HOME', 'DRAW', 'AWAY', 'OVER', 'UNDER', 'YES', 'NO')),
    label     TEXT NOT NULL,
    odd_x100  INTEGER NOT NULL CHECK (odd_x100 >= 101),
    result    TEXT NOT NULL DEFAULT 'PENDING' CHECK (result IN ('PENDING', 'WON', 'LOST', 'VOID')),
    UNIQUE (market_id, code)
);

-- ===== providers (Persona 2) =====
-- Contador diario persistido: corta las llamadas al llegar al límite, también tras reiniciar.
CREATE TABLE provider_usage (
    provider TEXT NOT NULL,
    day      DATE NOT NULL,
    requests INTEGER NOT NULL DEFAULT 0 CHECK (requests >= 0),
    PRIMARY KEY (provider, day)
);

-- ===== wallet (Persona 3) =====
-- balance_cents puede quedar negativo solo por una reversión del admin (RESETTLE);
-- los débitos normales usan UPDATE ... WHERE balance_cents >= monto.
CREATE TABLE wallets (
    user_id       uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    balance_cents BIGINT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ledger. Invariante: wallets.balance_cents = SUM(amount_cents) del usuario.
CREATE TABLE wallet_transactions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid NOT NULL REFERENCES wallets (user_id),
    type                TEXT NOT NULL CHECK (type IN (
                            'DEPOSIT_INITIAL', 'BET_STAKE', 'BET_PAYOUT', 'BET_REFUND',
                            'BET_PAYOUT_REVERSAL', 'BET_REFUND_REVERSAL', 'ADMIN_ADJUSTMENT')),
    amount_cents        BIGINT NOT NULL CHECK (amount_cents <> 0),
    balance_after_cents BIGINT NOT NULL,
    reference           TEXT NOT NULL, -- user:<id> | bet:<id> | bet:<id>:v<n> | admin:<uuid>
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT wallet_transactions_reference_type_key UNIQUE (reference, type),
    CHECK (
        (type IN ('DEPOSIT_INITIAL', 'BET_PAYOUT', 'BET_REFUND') AND amount_cents > 0) OR
        (type IN ('BET_STAKE', 'BET_PAYOUT_REVERSAL', 'BET_REFUND_REVERSAL') AND amount_cents < 0) OR
        (type = 'ADMIN_ADJUSTMENT')
    )
);
CREATE INDEX wallet_transactions_user_idx ON wallet_transactions (user_id, created_at DESC);

-- ===== betting / settlement (Persona 3) =====
CREATE TABLE bets (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                uuid NOT NULL REFERENCES users (id),
    idempotency_key        TEXT NOT NULL,
    stake_cents            BIGINT NOT NULL CHECK (stake_cents > 0),
    total_odd_x100         INTEGER NOT NULL CHECK (total_odd_x100 >= 101),
    potential_payout_cents BIGINT NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'WON', 'LOST', 'VOID')),
    payout_cents           BIGINT NOT NULL DEFAULT 0,
    settlement_version     INTEGER NOT NULL DEFAULT 0,
    placed_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at             TIMESTAMPTZ,
    CONSTRAINT bets_user_idempotency_key UNIQUE (user_id, idempotency_key)
);
CREATE INDEX bets_user_idx ON bets (user_id, placed_at DESC);
CREATE INDEX bets_pending_idx ON bets (id) WHERE status = 'PENDING';

-- odd_x100 queda congelada al apostar. UNIQUE(bet_id, event_id): una combinada
-- no puede tener dos selecciones del mismo partido.
CREATE TABLE bet_selections (
    bet_id       uuid NOT NULL REFERENCES bets (id) ON DELETE CASCADE,
    selection_id uuid NOT NULL REFERENCES selections (id),
    event_id     uuid NOT NULL REFERENCES events (id),
    odd_x100     INTEGER NOT NULL CHECK (odd_x100 >= 101),
    result       TEXT NOT NULL DEFAULT 'PENDING' CHECK (result IN ('PENDING', 'WON', 'LOST', 'VOID')),
    PRIMARY KEY (bet_id, selection_id),
    UNIQUE (bet_id, event_id)
);
CREATE INDEX bet_selections_event_idx ON bet_selections (event_id);

-- +goose Down
DROP TABLE bet_selections;
DROP TABLE bets;
DROP TABLE wallet_transactions;
DROP TABLE wallets;
DROP TABLE provider_usage;
DROP TABLE selections;
DROP TABLE markets;
DROP TABLE external_refs;
DROP TABLE events;
DROP TABLE teams;
DROP TABLE leagues;
DROP TABLE audit_log;
DROP TABLE refresh_tokens;
DROP TABLE users;
