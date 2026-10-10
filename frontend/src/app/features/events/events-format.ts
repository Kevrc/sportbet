import { League, Market, MarketCode, MatchStatus, SportEvent } from '../../core/api/models';

const STATUS_LABELS: Record<MatchStatus, string> = {
  SCHEDULED: 'Programado',
  LIVE: 'En vivo',
  INTERRUPTED: 'Interrumpido',
  FINISHED: 'Finalizado',
  POSTPONED: 'Aplazado',
  CANCELLED: 'Cancelado',
  ABANDONED: 'Abandonado',
  AWARDED: 'Adjudicado',
};

const MARKET_TITLES: Record<MarketCode, string> = {
  MATCH_RESULT: 'Resultado final (1X2)',
  TOTAL_GOALS_2_5: 'Total de goles (2.5)',
  BOTH_TEAMS_TO_SCORE: 'Ambos equipos anotan',
};

/** El mercado se cierra 60 s antes del inicio (regla de dominio). */
export const MARKET_CLOSE_MARGIN_MS = 60_000;

export function statusLabel(status: MatchStatus): string {
  return STATUS_LABELS[status] ?? status;
}

export function marketTitle(code: MarketCode): string {
  return MARKET_TITLES[code] ?? code;
}

/** "2 - 1", o null si el evento aún no tiene marcador. */
export function scoreText(e: Pick<SportEvent, 'homeScore' | 'awayScore'>): string | null {
  return e.homeScore != null && e.awayScore != null ? `${e.homeScore} - ${e.awayScore}` : null;
}

/**
 * Motivo por el que no se puede apostar en el mercado, o null si está abierto.
 * Es solo informativo: la validación real la hace el backend.
 */
export function marketLockReason(
  event: Pick<SportEvent, 'status' | 'startAt'>,
  market: Pick<Market, 'status'>,
  now = Date.now(),
): string | null {
  if (market.status === 'SETTLED') return 'Mercado liquidado';
  if (market.status === 'VOID') return 'Mercado anulado';
  if (market.status === 'CLOSED') return 'Mercado cerrado';
  if (event.status !== 'SCHEDULED') return 'El partido ya comenzó o terminó';
  if (now >= new Date(event.startAt).getTime() - MARKET_CLOSE_MARGIN_MS) {
    return 'Cerrado: el partido está por comenzar';
  }
  return null;
}

export interface LeagueGroup {
  league: League;
  events: SportEvent[];
}

/** Agrupa por liga: primero las destacadas, luego por nombre; dentro, por hora de inicio. */
export function groupByLeague(events: readonly SportEvent[]): LeagueGroup[] {
  const groups = new Map<string, LeagueGroup>();
  for (const e of events) {
    const g = groups.get(e.league.id) ?? { league: e.league, events: [] };
    g.events.push(e);
    groups.set(e.league.id, g);
  }
  const list = [...groups.values()];
  for (const g of list) {
    g.events.sort((a, b) => new Date(a.startAt).getTime() - new Date(b.startAt).getTime());
  }
  return list.sort(
    (a, b) =>
      Number(b.league.featured) - Number(a.league.featured) ||
      a.league.name.localeCompare(b.league.name),
  );
}
