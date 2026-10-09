import { Injectable, inject, signal } from '@angular/core';

import { WalletService } from './wallet.service';

/** Saldo compartido: lo refrescan la billetera y, tras apostar, el boleto. */
@Injectable({ providedIn: 'root' })
export class WalletStore {
  private readonly service = inject(WalletService);

  private readonly _balanceCents = signal<number | null>(null);
  readonly balanceCents = this._balanceCents.asReadonly();

  refresh(): void {
    this.service.get().subscribe({
      next: (w) => this._balanceCents.set(w.balanceCents),
      error: () => undefined, // se conserva el último saldo conocido
    });
  }
}
