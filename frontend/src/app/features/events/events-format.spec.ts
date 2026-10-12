import { event, league, market } from './events-testing';
import {
  formatRound,
  formatVenue,
  groupByLeague,
  marketLockReason,
  marketTitle,
  scoreText,
  statusLabel,
} from './events-format';

describe('statusLabel / marketTitle', () => {
  it('traduce estados y mercados', () => {
    expect(statusLabel('LIVE')).toBe('En vivo');
    expect(statusLabel('POSTPONED')).toBe('Aplazado');
    expect(marketTitle('MATCH_RESULT')).toBe('Resultado final (1X2)');
    expect(marketTitle('BOTH_TEAMS_TO_SCORE')).toBe('Ambos equipos anotan');
  });
});

describe('scoreText', () => {
  it('muestra el marcador solo si existen los dos goles', () => {
    expect(scoreText({ homeScore: 2, awayScore: 1 })).toBe('2 - 1');
    expect(scoreText({ homeScore: 0, awayScore: 0 })).toBe('0 - 0');
    expect(scoreText({ homeScore: null, awayScore: null })).toBeNull();
    expect(scoreText({ homeScore: 1, awayScore: null })).toBeNull();
    expect(scoreText({})).toBeNull();
  });
});

describe('marketLockReason', () => {
  const start = '2026-10-12T18:00:00Z';
  const t = (iso: string) => new Date(iso).getTime();
  const scheduled = { status: 'SCHEDULED' as const, startAt: start };

  it('un mercado abierto de un partido futuro no tiene bloqueo', () => {
    expect(marketLockReason(scheduled, market(), t('2026-10-12T12:00:00Z'))).toBeNull();
  });

  it('se cierra 60 s antes del inicio', () => {
    expect(marketLockReason(scheduled, market(), t('2026-10-12T17:58:59Z'))).toBeNull();
    expect(marketLockReason(scheduled, market(), t('2026-10-12T17:59:00Z'))).toContain(
      'por comenzar',
    );
    expect(marketLockReason(scheduled, market(), t('2026-10-12T19:00:00Z'))).toContain(
      'por comenzar',
    );
  });

  it('el estado del mercado manda sobre la hora', () => {
    const now = t('2026-10-12T12:00:00Z');
    expect(marketLockReason(scheduled, market({ status: 'CLOSED' }), now)).toBe('Mercado cerrado');
    expect(marketLockReason(scheduled, market({ status: 'SETTLED' }), now)).toBe(
      'Mercado liquidado',
    );
    expect(marketLockReason(scheduled, market({ status: 'VOID' }), now)).toBe('Mercado anulado');
  });

  it('un partido en vivo o terminado no admite apuestas aunque el mercado diga OPEN', () => {
    const now = t('2026-10-12T12:00:00Z');
    expect(marketLockReason({ status: 'LIVE', startAt: start }, market(), now)).not.toBeNull();
    expect(marketLockReason({ status: 'FINISHED', startAt: start }, market(), now)).not.toBeNull();
  });
});

describe('groupByLeague', () => {
  it('agrupa por liga, destacadas primero y luego por nombre', () => {
    const zeta = league('z', { name: 'Zeta' });
    const alfa = league('a', { name: 'Alfa' });
    const top = league('t', { name: 'Top', featured: true });
    const groups = groupByLeague([
      event('1', { league: zeta }),
      event('2', { league: alfa }),
      event('3', { league: top }),
      event('4', { league: zeta }),
    ]);
    expect(groups.map((g) => g.league.name)).toEqual(['Top', 'Alfa', 'Zeta']);
    expect(groups[2].events.map((e) => e.id)).toEqual(['1', '4']);
  });

  it('ordena los partidos de una liga por hora de inicio', () => {
    const groups = groupByLeague([
      event('tarde', { startAt: '2026-10-12T20:00:00Z' }),
      event('temprano', { startAt: '2026-10-12T10:00:00Z' }),
    ]);
    expect(groups[0].events.map((e) => e.id)).toEqual(['temprano', 'tarde']);
  });

  it('sin eventos no hay grupos', () => {
    expect(groupByLeague([])).toEqual([]);
  });
});

describe('formatRound y formatVenue', () => {
  it('traduce la jornada regular y deja legibles las demás', () => {
    expect(formatRound('Regular Season - 8')).toBe('Jornada 8');
    expect(formatRound('Clausura - 12')).toBe('Clausura · 12');
    expect(formatRound(null)).toBeNull();
  });

  it('arma estadio y ciudad', () => {
    expect(formatVenue({ name: 'Anfield', city: 'Liverpool' })).toBe('Anfield, Liverpool');
    expect(formatVenue({ name: 'Estadio Monumental', city: null })).toBe('Estadio Monumental');
    expect(formatVenue(null)).toBeNull();
  });
});
