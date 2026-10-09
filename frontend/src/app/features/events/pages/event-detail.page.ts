import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { inject } from '@angular/core';
import { takeUntilDestroyed, toObservable } from '@angular/core/rxjs-interop';
import { RouterLink } from '@angular/router';
import { catchError, of, switchMap, tap } from 'rxjs';

import { EventDetail, Market } from '../../../core/api/models';
import { ApiError } from '../../../core/http/error.interceptor';
import { formatOdd } from '../../betting/public-api';
import { marketLockReason, marketTitle, scoreText, statusLabel } from '../events-format';
import { EventsService } from '../events.service';
import { LiveScoresStore } from '../live-scores.store';

@Component({
  selector: 'app-event-detail-page',
  imports: [DatePipe, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './event-detail.page.html',
  styleUrl: './event-detail.page.scss',
  providers: [LiveScoresStore],
})
export class EventDetailPage {
  private readonly service = inject(EventsService);
  protected readonly live = inject(LiveScoresStore);

  /** Parámetro :id de la ruta (withComponentInputBinding). */
  readonly id = input.required<string>();

  protected readonly event = signal<EventDetail | null>(null);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  protected readonly notFound = signal(false);
  private readonly attempt = signal(0);

  /** El evento cargado con el marcador y estado en vivo encima. */
  protected readonly view = computed(() => {
    const e = this.event();
    return e ? this.live.apply(e) : null;
  });

  protected readonly score = computed(() => {
    const e = this.view();
    return e ? scoreText(e) : null;
  });

  protected readonly statusText = statusLabel;
  protected readonly title = marketTitle;
  protected readonly odd = formatOdd;

  constructor() {
    // Tras reconectar el SSE se pudo perder algún mensaje: se vuelve a pedir el evento.
    this.live.resynced$.pipe(takeUntilDestroyed()).subscribe(() => this.retry());

    toObservable(computed(() => ({ id: this.id(), attempt: this.attempt() })))
      .pipe(
        tap(() => {
          this.loading.set(true);
          this.error.set(null);
          this.notFound.set(false);
        }),
        switchMap(({ id }) =>
          this.service.get(id).pipe(
            catchError((err: unknown) => {
              if (err instanceof ApiError && err.status === 404) this.notFound.set(true);
              else this.error.set('No se pudo cargar el evento.');
              return of(null);
            }),
          ),
        ),
        takeUntilDestroyed(),
      )
      .subscribe((e) => {
        this.event.set(e);
        this.loading.set(false);
      });
  }

  protected retry(): void {
    this.attempt.update((n) => n + 1);
  }

  /** Motivo de bloqueo del mercado, calculado al renderizar. */
  protected lockReason(market: Market): string | null {
    const e = this.view();
    return e ? marketLockReason(e, market) : 'Mercado cerrado';
  }
}
