import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { WalletTransaction } from '../../../core/api/models';
import { formatCents } from '../money';
import { signedAmount, txLabel } from '../wallet-format';
import { WalletService } from '../wallet.service';
import { WalletStore } from '../wallet.store';

const PAGE_SIZE = 20;

@Component({
  selector: 'app-wallet-page',
  imports: [DatePipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './wallet.page.html',
  styleUrl: './wallet.page.scss',
})
export class WalletPage {
  private readonly service = inject(WalletService);
  private readonly wallet = inject(WalletStore);

  protected readonly balance = this.wallet.balanceCents;
  protected readonly txs = signal<WalletTransaction[]>([]);
  protected readonly page = signal(1);
  protected readonly total = signal(0);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  protected readonly totalPages = computed(() => Math.max(1, Math.ceil(this.total() / PAGE_SIZE)));

  protected readonly money = formatCents;
  protected readonly label = txLabel;
  protected readonly signed = (c: number) => signedAmount(c, formatCents);

  constructor() {
    this.load(1);
  }

  protected load(page: number): void {
    this.loading.set(true);
    this.error.set(null);
    this.wallet.refresh();
    this.service.transactions(page, PAGE_SIZE).subscribe({
      next: (res) => {
        this.txs.set(res.items);
        this.total.set(res.total);
        this.page.set(res.page);
        this.loading.set(false);
      },
      error: () => {
        this.error.set('No se pudieron cargar los movimientos.');
        this.loading.set(false);
      },
    });
  }
}
