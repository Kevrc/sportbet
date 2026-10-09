import { signal } from '@angular/core';
import { Subject } from 'rxjs';

import {
  EventDetail,
  LiveMessages,
  Market,
  MatchFinishedMessage,
  MatchStartedMessage,
  MatchUpdatedMessage,
  SportEvent,
} from '../../core/api/models';

/** LiveEventsService falso: las pruebas emiten los mensajes SSE a mano (jsdom no tiene EventSource). */
export function fakeLive() {
  const updated = new Subject<MatchUpdatedMessage>();
  const started = new Subject<MatchStartedMessage>();
  const finished = new Subject<MatchFinishedMessage>();
  const reconnected = new Subject<void>();
  const streams = {
    'match.updated': updated,
    'match.started': started,
    'match.finished': finished,
  };
  return {
    connected: signal(true),
    reconnected$: reconnected.asObservable(),
    on: <T extends keyof LiveMessages>(type: T) => streams[type].asObservable(),
    isStale: (iso: string | null | undefined, now = Date.now()) =>
      !iso || now - Date.parse(iso) > 5 * 60 * 1000,
    emit: {
      updated: (m: MatchUpdatedMessage) => updated.next(m),
      started: (m: MatchStartedMessage) => started.next(m),
      finished: (m: MatchFinishedMessage) => finished.next(m),
    },
    reconnect: () => reconnected.next(),
  };
}

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
