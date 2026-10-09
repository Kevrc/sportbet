import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiClient } from '../../core/api/api-client';
import { Bet, BetStatus, Market, Paged, PlaceBetRequest } from '../../core/api/models';

/** POST/GET /bets, GET /bets/{id} y los mercados vigentes de un evento. */
@Injectable({ providedIn: 'root' })
export class BettingService {
  private readonly api = inject(ApiClient);

  /** El mismo idempotencyKey devuelve la apuesta original en vez de crear otra. */
  placeBet(body: PlaceBetRequest, idempotencyKey: string): Observable<Bet> {
    return this.api.post<Bet>('/bets', body, { idempotencyKey });
  }

  list(params: { status?: BetStatus; page: number; pageSize: number }): Observable<Paged<Bet>> {
    return this.api.get<Paged<Bet>>('/bets', params);
  }

  get(id: string): Observable<Bet> {
    return this.api.get<Bet>(`/bets/${encodeURIComponent(id)}`);
  }

  /** Cuotas y estado actuales: se usan para refrescar el boleto tras un ODDS_CHANGED. */
  eventMarkets(eventId: string): Observable<Market[]> {
    return this.api.get<Market[]>(`/events/${encodeURIComponent(eventId)}/markets`);
  }
}
