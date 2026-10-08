import { Injectable, NgZone, inject, signal } from '@angular/core';
import { Observable, Subject, filter, map, share } from 'rxjs';

import { environment } from '../../../environments/environment';
import { LiveMessages } from '../api/models';

type LiveType = keyof LiveMessages;
interface RawMessage {
  type: LiveType;
  data: unknown;
}

/**
 * Una sola conexión SSE a /events/stream, abierta mientras haya suscriptores.
 * EventSource no envía cabeceras, por eso el stream es público.
 * Al reconectar emite reconnected$ para que la vista vuelva a pedir GET /events/live.
 */
@Injectable({ providedIn: 'root' })
export class LiveEventsService {
  private readonly zone = inject(NgZone);
  private readonly types: LiveType[] = ['match.updated', 'match.started', 'match.finished'];

  readonly connected = signal(false);
  readonly lastMessageAt = signal<Date | null>(null);
  private readonly reconnectedSubject = new Subject<void>();
  readonly reconnected$ = this.reconnectedSubject.asObservable();

  private readonly stream$ = new Observable<RawMessage>((subscriber) => {
    const source = new EventSource(`${environment.apiUrl}/events/stream`);
    let opened = false;
    source.onopen = () => {
      this.connected.set(true);
      if (opened) this.reconnectedSubject.next();
      opened = true;
    };
    source.onerror = () => this.connected.set(false);
    const listeners = this.types.map((type) => {
      const fn = (e: MessageEvent<string>) =>
        this.zone.run(() => {
          this.lastMessageAt.set(new Date());
          subscriber.next({ type, data: JSON.parse(e.data) });
        });
      source.addEventListener(type, fn);
      return [type, fn] as const;
    });
    return () => {
      listeners.forEach(([t, fn]) => source.removeEventListener(t, fn));
      source.close();
      this.connected.set(false);
    };
  }).pipe(share());

  on<T extends LiveType>(type: T): Observable<LiveMessages[T]> {
    return this.stream$.pipe(
      filter((m) => m.type === type),
      map((m) => m.data as LiveMessages[T]),
    );
  }

  /** true si el dato en vivo tiene más de 5 minutos. */
  isStale(liveUpdatedAt: string | null | undefined, now = Date.now()): boolean {
    if (!liveUpdatedAt) return true;
    return now - new Date(liveUpdatedAt).getTime() > environment.liveStaleAfterMs;
  }
}
