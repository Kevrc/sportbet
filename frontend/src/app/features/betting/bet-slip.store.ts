import { Injectable, computed, inject, signal } from '@angular/core';
import { catchError, forkJoin, of } from 'rxjs';

import { Bet, Market, MarketCode } from '../../core/api/models';
import { WalletStore, parseToCents } from '../wallet/public-api';
import { BetFailure, describeBetError } from './bet-errors';
import { BettingService } from './betting.service';
import {
  MAX_PAYOUT_CENTS,
  MAX_SELECTIONS,
  MAX_STAKE_CENTS,
  MAX_TOTAL_ODD_X100,
  MIN_STAKE_CENTS,
  combineOdds,
  potentialPayoutCents,
} from './odds';

/** Lo que una página entrega al boleto cuando el usuario toca una cuota. */
export interface SlipPick {
  selectionId: string;
  eventId: string;
  eventName: string;
  marketId: string;
  marketCode: MarketCode;
  /** Nombre legible del mercado, p. ej. "Resultado final (1X2)". */
  marketLabel: string;
  label: string;
  oddX100: number;
}

export interface SlipSelection extends SlipPick {
  /** ok: cuota vigente · changed: cambió y falta confirmar · closed: el mercado ya no admite apuestas. */
  status: 'ok' | 'changed' | 'closed';
  /** Cuota que vio el usuario antes del cambio (solo con status "changed"). */
  previousOddX100?: number;
}

export type PickResult = 'added' | 'removed' | 'replaced' | 'same-event' | 'full';

export type PlaceResult = { kind: 'success'; bet: Bet } | { kind: 'error'; failure: BetFailure };

function newIdempotencyKey(): string {
  return (
    globalThis.crypto?.randomUUID?.() ?? `k-${Date.now()}-${Math.random().toString(36).slice(2)}`
  );
}

/**
 * Boleto de apuestas: simple (1 selección) o combinada (2 a 5, nunca dos del mismo partido).
 * Vive durante toda la sesión de la SPA, así sobrevive a ir a iniciar sesión y volver.
 */
@Injectable({ providedIn: 'root' })
export class BetSlipStore {
  private readonly service = inject(BettingService);
  private readonly wallet = inject(WalletStore);

  private readonly _selections = signal<SlipSelection[]>([]);
  private readonly _stakeText = signal('');
  private readonly _placing = signal(false);
  private readonly _result = signal<PlaceResult | null>(null);
  private readonly _notice = signal<string | null>(null);
  private idempotencyKey: string | null = null;

  readonly selections = this._selections.asReadonly();
  readonly stakeText = this._stakeText.asReadonly();
  readonly placing = this._placing.asReadonly();
  readonly result = this._result.asReadonly();
  /** Aviso de una selección rechazada en el cliente (mismo partido, boleto lleno). */
  readonly notice = this._notice.asReadonly();

  readonly count = computed(() => this._selections().length);
  readonly selectedIds = computed(() => new Set(this._selections().map((s) => s.selectionId)));
  readonly hasChanged = computed(() => this._selections().some((s) => s.status === 'changed'));
  readonly hasClosed = computed(() => this._selections().some((s) => s.status === 'closed'));

  readonly stakeCents = computed(() => parseToCents(this._stakeText()));
  readonly totalOddX100 = computed(() => combineOdds(this._selections().map((s) => s.oddX100)));
  readonly oddCapped = computed(() => this.totalOddX100() >= MAX_TOTAL_ODD_X100);
  readonly payoutCents = computed(() => {
    const stake = this.stakeCents();
    return stake === null ? null : potentialPayoutCents(stake, this.totalOddX100());
  });

  /** Mensaje de validación del stake, o null si está vacío o es válido. */
  readonly stakeError = computed(() => {
    const text = this._stakeText().trim();
    if (!text) return null;
    const stake = this.stakeCents();
    if (stake === null) return 'Escribe un monto válido, con máximo 2 decimales.';
    if (stake < MIN_STAKE_CENTS) return 'El monto mínimo es 1.00.';
    if (stake > MAX_STAKE_CENTS) return 'El monto máximo es 500.00.';
    const balance = this.wallet.balanceCents();
    if (balance !== null && stake > balance) return 'No tienes saldo suficiente.';
    return null;
  });

  /** Aviso si el pago potencial supera el tope (el backend rechazaría la apuesta). */
  readonly payoutError = computed(() => {
    const payout = this.payoutCents();
    return payout !== null && payout > MAX_PAYOUT_CENTS
      ? 'El pago potencial supera el máximo de 10 000.00. Baja el monto o quita selecciones.'
      : null;
  });

  readonly canPlace = computed(
    () =>
      this.count() > 0 &&
      this.stakeCents() !== null &&
      this.stakeError() === null &&
      this.payoutError() === null &&
      !this.hasChanged() &&
      !this.hasClosed() &&
      !this._placing(),
  );

  // ---- selecciones ----

  /** Añade, quita o reemplaza según el estado del boleto. No lanza: devuelve qué pasó. */
  toggle(pick: SlipPick): PickResult {
    const list = this._selections();
    if (list.some((s) => s.selectionId === pick.selectionId)) {
      this.mutate(list.filter((s) => s.selectionId !== pick.selectionId));
      return 'removed';
    }
    // Otra opción del mismo mercado reemplaza a la anterior.
    const sameMarket = list.findIndex((s) => s.marketId === pick.marketId);
    if (sameMarket >= 0) {
      this.mutate(list.map((s, i) => (i === sameMarket ? { ...pick, status: 'ok' } : s)));
      return 'replaced';
    }
    if (list.some((s) => s.eventId === pick.eventId)) {
      this._notice.set('No puedes combinar dos selecciones del mismo partido.');
      return 'same-event';
    }
    if (list.length >= MAX_SELECTIONS) {
      this._notice.set(`Una combinada admite máximo ${MAX_SELECTIONS} selecciones.`);
      return 'full';
    }
    this.mutate([...list, { ...pick, status: 'ok' }]);
    return 'added';
  }

  remove(selectionId: string): void {
    this.mutate(this._selections().filter((s) => s.selectionId !== selectionId));
  }

  /** Vacía el boleto (selecciones y monto). */
  clear(): void {
    this._stakeText.set('');
    this.mutate([]);
  }

  setStake(text: string): void {
    this._stakeText.set(text);
    this.touch();
  }

  /** El usuario acepta las cuotas nuevas: quedan como las que enviará. */
  acceptChanges(): void {
    this.mutate(
      this._selections().map((s) =>
        s.status === 'changed' ? { ...s, status: 'ok', previousOddX100: undefined } : s,
      ),
    );
  }

  dismissResult(): void {
    this._result.set(null);
  }

  dismissNotice(): void {
    this._notice.set(null);
  }

  // ---- apostar ----

  place(): void {
    if (!this.canPlace()) return;
    const stakeCents = this.stakeCents()!;
    this._placing.set(true); // síncrono: un segundo clic ya no pasa canPlace()
    this._result.set(null);
    this.idempotencyKey ??= newIdempotencyKey();

    this.service
      .placeBet(
        {
          stakeCents,
          selections: this._selections().map((s) => ({
            selectionId: s.selectionId,
            oddX100: s.oddX100,
          })),
        },
        this.idempotencyKey,
      )
      .subscribe({
        next: (bet) => {
          this._placing.set(false);
          this._stakeText.set('');
          this._selections.set([]);
          this._notice.set(null);
          this.idempotencyKey = null;
          this._result.set({ kind: 'success', bet });
          this.wallet.refresh();
        },
        error: (err: unknown) => this.onPlaceError(describeBetError(err)),
      });
  }

  /** Aplica cuotas y estados vigentes a las selecciones del boleto. */
  applyMarkets(markets: readonly Market[]): void {
    const current = new Map<string, { odd: number; open: boolean }>();
    for (const m of markets) {
      for (const sel of m.selections) {
        current.set(sel.id, { odd: sel.oddX100, open: m.status === 'OPEN' });
      }
    }
    this.mutate(
      this._selections().map((s) => {
        const now = current.get(s.selectionId);
        if (!now) return s;
        if (!now.open) return { ...s, status: 'closed' };
        if (now.odd !== s.oddX100) {
          return { ...s, status: 'changed', previousOddX100: s.oddX100, oddX100: now.odd };
        }
        return s;
      }),
      false,
    );
  }

  // ---- internos ----

  private onPlaceError(failure: BetFailure): void {
    this._placing.set(false);
    if (!failure.keepKey) this.idempotencyKey = null;
    this._result.set({ kind: 'error', failure });
    if (failure.kind === 'odds-changed' || failure.kind === 'market-closed') this.refreshOdds();
    if (failure.kind === 'insufficient-funds') this.wallet.refresh();
  }

  private refreshOdds(): void {
    const eventIds = [...new Set(this._selections().map((s) => s.eventId))];
    if (eventIds.length === 0) return;
    forkJoin(
      eventIds.map((id) =>
        this.service.eventMarkets(id).pipe(catchError(() => of([] as Market[]))),
      ),
    ).subscribe((lists) => this.applyMarkets(lists.flat()));
  }

  /** Cambia las selecciones. Todo cambio invalida la clave de idempotencia y el resultado previo. */
  private mutate(next: SlipSelection[], clearResult = true): void {
    this._selections.set(next);
    this._notice.set(null);
    this.touch(clearResult);
  }

  /** El cuerpo de la apuesta cambió: la próxima es otra y necesita otra clave. */
  private touch(clearResult = true): void {
    this.idempotencyKey = null;
    if (clearResult) this._result.set(null);
  }
}
