import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { RouterLink } from '@angular/router';
import { Subject, catchError, of, startWith, switchMap, tap } from 'rxjs';

import { League, MatchStatus, SportEvent } from '../../../core/api/models';
import { groupByLeague, scoreText, statusLabel } from '../events-format';
import { EventFilters, EventsService } from '../events.service';
import { LiveScoresStore } from '../live-scores.store';
import { TeamCrest } from '../team-crest';

const STATUS_FILTERS: { value: MatchStatus; label: string }[] = [
  { value: 'SCHEDULED', label: 'Programados' },
  { value: 'LIVE', label: 'En vivo' },
  { value: 'FINISHED', label: 'Finalizados' },
];

@Component({
  selector: 'app-event-list-page',
  imports: [DatePipe, RouterLink, TeamCrest],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './event-list.page.html',
  styleUrl: './event-list.page.scss',
  providers: [LiveScoresStore],
})
export class EventListPage {
  private readonly service = inject(EventsService);
  protected readonly live = inject(LiveScoresStore);

  protected readonly statusFilters = STATUS_FILTERS;
  protected readonly leagues = signal<League[]>([]);
  protected readonly filters = signal<EventFilters>({});
  protected readonly events = signal<SportEvent[]>([]);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);

  protected readonly groups = computed(() =>
    groupByLeague(this.events().map((e) => this.live.apply(e))),
  );
  protected readonly hasFilters = computed(() => Object.values(this.filters()).some(Boolean));

  protected readonly statusText = statusLabel;
  protected readonly score = scoreText;

  private readonly reload$ = new Subject<void>();

  constructor() {
    // Tras reconectar el SSE se pudo perder algún mensaje: se vuelve a pedir la lista.
    this.live.resynced$.pipe(takeUntilDestroyed()).subscribe(() => this.reload());

    this.service
      .leagues()
      .pipe(
        catchError(() => of([] as League[])), // sin ligas el filtro queda vacío, la lista sigue
        takeUntilDestroyed(),
      )
      .subscribe((l) => this.leagues.set(l));

    // switchMap descarta la respuesta anterior si el usuario cambia otro filtro.
    this.reload$
      .pipe(
        startWith(undefined),
        tap(() => {
          this.loading.set(true);
          this.error.set(null);
        }),
        switchMap(() =>
          this.service.list(this.filters()).pipe(
            catchError(() => {
              this.error.set('No se pudieron cargar los eventos.');
              return of(null);
            }),
          ),
        ),
        takeUntilDestroyed(),
      )
      .subscribe((events) => {
        if (events) this.events.set(events);
        this.loading.set(false);
      });
  }

  protected reload(): void {
    this.reload$.next();
  }

  protected setFilter<K extends keyof EventFilters>(key: K, value: EventFilters[K] | ''): void {
    this.filters.update((f) => ({ ...f, [key]: value === '' ? undefined : value }));
    this.reload();
  }

  protected clearFilters(): void {
    this.filters.set({});
    this.reload();
  }
}
