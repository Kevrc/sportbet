import { Injectable, inject, signal } from '@angular/core';

import { WalletService } from './wallet.service';

/** Saldo compartido: lo refrescan la billetera y, tras apostar, el boleto. */
@Injectable({ providedIn: 'root' })
export class WalletStore {
  private readonly service = inject(WalletService);

  private readonly _balanceCents = signal<number | null>(null);
  readonly balanceCents = this._balanceCents.asReadonly();
  private readonly _failed = signal(false);
  /** true si la última carga del saldo falló (el último saldo conocido, si hay, se conserva). */
  readonly failed = this._failed.asReadonly();

  refresh(): void {
    this.service.get().subscribe({
      next: (w) => {
        this._balanceCents.set(w.balanceCents);
        this._failed.set(false);
      },
      error: () => this._failed.set(true),
    });
  }
}
