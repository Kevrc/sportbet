import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { RouterLink } from '@angular/router';
import { Subject, catchError, of, startWith, switchMap, tap } from 'rxjs';

import { Bet, BetStatus } from '../../../core/api/models';
import { formatCents } from '../../wallet/public-api';
import {
  betKind,
  betOutcome,
  betStatusLabel,
  marketName,
  selectionResultLabel,
} from '../bet-format';
import { BettingService } from '../betting.service';
import { formatOdd } from '../odds';

const PAGE_SIZE = 10;

const STATUS_FILTERS: { value: BetStatus; label: string }[] = [
  { value: 'PENDING', label: 'Pendientes' },
  { value: 'WON', label: 'Ganadas' },
  { value: 'LOST', label: 'Perdidas' },
  { value: 'VOID', label: 'Anuladas' },
];

@Component({
  selector: 'app-bet-history-page',
  imports: [DatePipe, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './bet-history.page.html',
  styleUrl: './bet-history.page.scss',
})
export class BetHistoryPage {
  private readonly service = inject(BettingService);

  protected readonly statusFilters = STATUS_FILTERS;
  protected readonly status = signal<BetStatus | undefined>(undefined);
  protected readonly page = signal(1);
  protected readonly bets = signal<Bet[]>([]);
  protected readonly total = signal(0);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  protected readonly totalPages = computed(() => Math.max(1, Math.ceil(this.total() / PAGE_SIZE)));

  protected readonly money = formatCents;
  protected readonly odd = formatOdd;
  protected readonly kind = betKind;
  protected readonly statusText = betStatusLabel;
  protected readonly resultText = selectionResultLabel;
  protected readonly market = marketName;
  protected readonly outcome = betOutcome;

  private readonly load$ = new Subject<void>();

  constructor() {
    // switchMap descarta la respuesta anterior si el usuario cambia de filtro o de página.
    this.load$
      .pipe(
        startWith(undefined),
        tap(() => {
          this.loading.set(true);
          this.error.set(null);
        }),
        switchMap(() =>
          this.service.list({ status: this.status(), page: this.page(), pageSize: PAGE_SIZE }).pipe(
            catchError(() => {
              this.error.set('No se pudo cargar tu historial de apuestas.');
              return of(null);
            }),
          ),
        ),
        takeUntilDestroyed(),
      )
      .subscribe((res) => {
        if (res) {
          this.bets.set(res.items ?? []);
          this.total.set(res.total);
          this.page.set(res.page);
        }
        this.loading.set(false);
      });
  }

  protected reload(): void {
    this.load$.next();
  }

  protected goTo(page: number): void {
    this.page.set(page);
    this.reload();
  }

  protected setStatus(value: string): void {
    this.status.set(value === '' ? undefined : (value as BetStatus));
    this.page.set(1);
    this.reload();
  }
}
