import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { Router, RouterLink } from '@angular/router';

import { AuthStore } from '../../../core/auth/auth.store';
import { WalletStore, formatCents } from '../../wallet/public-api';
import { BetSlipStore } from '../bet-slip.store';
import { formatOdd } from '../odds';

/** Boleto de apuestas: selecciones, monto, cuota total, pago potencial y botón de apostar. */
@Component({
  selector: 'app-bet-slip',
  imports: [RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './bet-slip.component.html',
  styleUrl: './bet-slip.component.scss',
})
export class BetSlipComponent {
  protected readonly slip = inject(BetSlipStore);
  private readonly auth = inject(AuthStore);
  private readonly wallet = inject(WalletStore);
  private readonly router = inject(Router);

  protected readonly loggedIn = this.auth.isAuthenticated;
  protected readonly balance = this.wallet.balanceCents;
  protected readonly money = formatCents;
  protected readonly odd = formatOdd;

  protected readonly kind = computed(() =>
    this.slip.count() > 1 ? `Combinada (${this.slip.count()})` : 'Simple',
  );

  /** Errores por campo del 400, listos para mostrar. */
  protected readonly fieldErrors = computed(() => {
    const r = this.slip.result();
    if (r?.kind !== 'error' || !r.failure.fields) return [];
    return Object.entries(r.failure.fields).map(([field, message]) => ({ field, message }));
  });

  constructor() {
    if (this.auth.isAuthenticated()) this.wallet.refresh();
  }

  protected onStake(event: Event): void {
    this.slip.setStake((event.target as HTMLInputElement).value);
  }

  protected goToLogin(): void {
    // El boleto vive en un store de sesión: sobrevive a ir a iniciar sesión y volver.
    void this.router.navigate(['/auth/login'], { queryParams: { returnUrl: this.router.url } });
  }
}
