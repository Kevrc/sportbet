import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { forkJoin } from 'rxjs';

import { AdminStats, AuditEntry } from '../../core/api/models';
import { formatCents } from '../wallet/public-api';
import { describeActivity } from './activity';
import { AdminUsersService } from './admin-users.service';

const ACTIVITY_SIZE = 10;

/**
 * Resumen del panel: cifras de apuestas y actividad reciente. Los datos salen
 * de la auditoría, que se llena con BetPlaced y BetSettled del bus.
 */
@Component({
  selector: 'app-admin-overview',
  imports: [DatePipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './admin-overview.html',
  styleUrl: './admin-overview.scss',
})
export class AdminOverview {
  private readonly service = inject(AdminUsersService);

  protected readonly stats = signal<AdminStats | null>(null);
  protected readonly activity = signal<AuditEntry[]>([]);
  protected readonly error = signal<string | null>(null);
  protected readonly loading = signal(true);
  protected readonly activityTotal = signal(0);
  protected readonly loadingMore = signal(false);

  protected readonly money = formatCents;
  protected readonly describe = describeActivity;
  /** Lo que se quedó la casa: apostado menos pagado. */
  protected readonly houseResult = computed(() => {
    const s = this.stats();
    return s ? s.stakeCents - s.payoutCents : 0;
  });

  constructor() {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.error.set(null);
    forkJoin({
      stats: this.service.stats(),
      audit: this.service.audit(1, ACTIVITY_SIZE),
    }).subscribe({
      next: ({ stats, audit }) => {
        this.stats.set(stats);
        this.activity.set(audit.items);
        this.activityTotal.set(audit.total);
        this.loading.set(false);
      },
      error: () => {
        this.error.set('No se pudieron cargar las estadísticas.');
        this.loading.set(false);
      },
    });
  }

  loadMore(): void {
    const page = Math.floor(this.activity().length / ACTIVITY_SIZE) + 1;
    this.loadingMore.set(true);
    this.service.audit(page, ACTIVITY_SIZE).subscribe({
      next: (audit) => {
        this.activity.update((items) => [...items, ...audit.items]);
        this.activityTotal.set(audit.total);
        this.loadingMore.set(false);
      },
      error: () => this.loadingMore.set(false),
    });
  }
}
