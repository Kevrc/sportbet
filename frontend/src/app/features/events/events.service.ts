import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiClient } from '../../core/api/api-client';
import { EventDetail, League, MatchStatus, SportEvent } from '../../core/api/models';

export interface EventFilters {
  /** yyyy-MM-dd */
  date?: string;
  leagueId?: string;
  status?: MatchStatus;
  featured?: boolean;
}

/** GET /events, /leagues y /events/{id}. */
@Injectable({ providedIn: 'root' })
export class EventsService {
  private readonly api = inject(ApiClient);

  list(filters: EventFilters = {}): Observable<SportEvent[]> {
    return this.api.get<SportEvent[]>('/events', {
      date: filters.date,
      leagueId: filters.leagueId,
      status: filters.status,
      // "featured=false" no significa "solo no destacados": se omite.
      featured: filters.featured ? true : undefined,
    });
  }

  leagues(): Observable<League[]> {
    return this.api.get<League[]>('/leagues');
  }

  get(id: string): Observable<EventDetail> {
    return this.api.get<EventDetail>(`/events/${encodeURIComponent(id)}`);
  }
}
