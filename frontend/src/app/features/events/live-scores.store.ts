import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { Subject, catchError, of } from 'rxjs';

import {
  MatchFinishedMessage,
  MatchStartedMessage,
  MatchStatus,
  MatchUpdatedMessage,
  SportEvent,
} from '../../core/api/models';
import { LiveEventsService } from '../../core/live/live-events.service';
import { EventsService } from './events.service';

/** Último estado en vivo conocido de un partido (viene del SSE o de GET /events/live). */
export interface LiveState {
  status: MatchStatus;
  homeScore: number | null;
  awayScore: number | null;
  minute: number | null;
  liveUpdatedAt: string;
}

const ACTIVE: MatchStatus[] = ['SCHEDULED', 'LIVE', 'INTERRUPTED'];
const TICK_MS = 30_000;

const time = (iso: string | null | undefined) => (iso ? Date.parse(iso) : 0);

/**
 * Marcadores en vivo para una página. Se provee a nivel de componente, así la
 * conexión SSE se cierra al salir de la página.
 *
 * - Aplica match.updated / match.started / match.finished sobre los eventos cargados.
 * - Al abrir y al reconectar pide GET /events/live; después de reconectar avisa con
 *   resynced$ para que la página vuelva a pedir sus datos (el bus puede perder mensajes).
 * - Ignora mensajes más viejos que el estado que ya tiene, y no mueve un partido
 *   terminado (los estados terminales no retroceden).
 */
@Injectable()
export class LiveScoresStore {
  private readonly live = inject(LiveEventsService);
  private readonly events = inject(EventsService);

  private readonly states = signal<ReadonlyMap<string, LiveState>>(new Map());
  private readonly now = signal(Date.now());
  private readonly resynced = new Subject<void>();

  readonly connected = this.live.connected;
  readonly resynced$ = this.resynced.asObservable();

  constructor() {
    const destroyed = inject(DestroyRef);
    const subs = [
      this.live.on('match.updated').subscribe((m) => this.onUpdated(m)),
      this.live.on('match.started').subscribe((m) => this.onStarted(m)),
      this.live.on('match.finished').subscribe((m) => this.onFinished(m)),
      this.live.reconnected$.subscribe(() => {
        this.loadLive();
        this.resynced.next();
      }),
    ];
    // Reloj para que "Desactualizado" aparezca solo, sin esperar otro mensaje.
    const timer = setInterval(() => this.now.set(Date.now()), TICK_MS);
    destroyed.onDestroy(() => {
      subs.forEach((s) => s.unsubscribe());
      clearInterval(timer);
    });
    this.loadLive();
  }

  /** El evento con el estado en vivo encima, si es más nuevo que el que ya trae. */
  apply<T extends SportEvent>(e: T): T {
    const s = this.states().get(e.id);
    if (!s || time(e.liveUpdatedAt) >= time(s.liveUpdatedAt)) return e;
    return { ...e, ...s };
  }

  /** true si el partido está en vivo y su último dato tiene más de 5 minutos. */
  isStale(e: Pick<SportEvent, 'status' | 'liveUpdatedAt'>): boolean {
    return e.status === 'LIVE' && this.live.isStale(e.liveUpdatedAt, this.now());
  }

  private loadLive(): void {
    this.events
      .live()
      .pipe(catchError(() => of([] as SportEvent[]))) // sin /events/live se sigue con el SSE
      .subscribe((list) => {
        for (const e of list) {
          this.put(e.id, {
            status: e.status,
            homeScore: e.homeScore ?? null,
            awayScore: e.awayScore ?? null,
            minute: e.minute ?? null,
            liveUpdatedAt: e.liveUpdatedAt ?? '',
          });
        }
      });
  }

  private onUpdated(m: MatchUpdatedMessage): void {
    this.put(m.eventId, {
      status: m.status,
      homeScore: m.homeScore,
      awayScore: m.awayScore,
      minute: m.minute,
      liveUpdatedAt: m.updatedAt,
    });
  }

  private onStarted(m: MatchStartedMessage): void {
    const cur = this.states().get(m.eventId);
    this.put(m.eventId, {
      status: 'LIVE',
      homeScore: cur?.homeScore ?? 0,
      awayScore: cur?.awayScore ?? 0,
      minute: cur?.minute ?? 0,
      liveUpdatedAt: m.startedAt,
    });
  }

  private onFinished(m: MatchFinishedMessage): void {
    this.put(m.eventId, {
      status: m.status,
      homeScore: m.homeScore,
      awayScore: m.awayScore,
      minute: null,
      liveUpdatedAt: m.finishedAt,
    });
  }

  private put(id: string, next: LiveState): void {
    const cur = this.states().get(id);
    if (cur) {
      if (!ACTIVE.includes(cur.status)) return;
      if (time(next.liveUpdatedAt) < time(cur.liveUpdatedAt)) return;
    }
    this.states.update((m) => new Map(m).set(id, next));
  }
}
