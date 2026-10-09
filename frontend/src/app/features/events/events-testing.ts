import { EventDetail, Market, SportEvent } from '../../core/api/models';

// Datos de prueba compartidos por las pruebas del feature.
export const league = (id = 'l1', extra: Partial<SportEvent['league']> = {}) => ({
  id,
  name: `Liga ${id}`,
  country: 'España',
  featured: false,
  ...extra,
});

export const event = (id: string, extra: Partial<SportEvent> = {}): SportEvent => ({
  id,
  league: league(),
  home: { id: `${id}h`, name: `Local ${id}` },
  away: { id: `${id}a`, name: `Visita ${id}` },
  startAt: '2026-10-12T18:00:00Z',
  status: 'SCHEDULED',
  homeScore: null,
  awayScore: null,
  minute: null,
  featured: false,
  ...extra,
});

export const market = (extra: Partial<Market> = {}): Market => ({
  id: 'm1',
  eventId: 'e1',
  code: 'MATCH_RESULT',
  status: 'OPEN',
  selections: [
    { id: 's1', code: 'HOME', label: 'Local', oddX100: 214, result: 'PENDING' },
    { id: 's2', code: 'DRAW', label: 'Empate', oddX100: 330, result: 'PENDING' },
    { id: 's3', code: 'AWAY', label: 'Visita', oddX100: 197, result: 'PENDING' },
  ],
  ...extra,
});

export const detail = (extra: Partial<EventDetail> = {}): EventDetail => ({
  ...event('e1'),
  markets: [market()],
  ...extra,
});
