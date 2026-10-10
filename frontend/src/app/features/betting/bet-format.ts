import { Bet, BetStatus, MarketCode, SelectionResult } from '../../core/api/models';

const STATUS_LABELS: Record<BetStatus, string> = {
  PENDING: 'Pendiente',
  WON: 'Ganada',
  LOST: 'Perdida',
  VOID: 'Anulada',
};

const RESULT_LABELS: Record<SelectionResult, string> = {
  PENDING: 'Pendiente',
  WON: 'Ganada',
  LOST: 'Perdida',
  VOID: 'Anulada (cuenta como 1.00)',
};

const MARKET_NAMES: Record<MarketCode, string> = {
  MATCH_RESULT: 'Resultado final (1X2)',
  TOTAL_GOALS_2_5: 'Total de goles (2.5)',
  BOTH_TEAMS_TO_SCORE: 'Ambos equipos anotan',
};

export function betStatusLabel(status: BetStatus): string {
  return STATUS_LABELS[status] ?? status;
}

export function selectionResultLabel(result: SelectionResult): string {
  return RESULT_LABELS[result] ?? result;
}

export function marketName(code: MarketCode): string {
  return MARKET_NAMES[code] ?? code;
}

/** "Simple" con una selección, "Combinada (3)" con varias. */
export function betKind(selectionCount: number): string {
  return selectionCount > 1 ? `Combinada (${selectionCount})` : 'Simple';
}

export interface BetOutcome {
  label: string;
  /** null cuando no hay monto que mostrar. */
  cents: number | null;
}

/** Qué cobra (o cobraría) la apuesta según su estado. */
export function betOutcome(
  bet: Pick<Bet, 'status' | 'potentialPayoutCents' | 'payoutCents'>,
): BetOutcome {
  switch (bet.status) {
    case 'PENDING':
      return { label: 'Pago potencial', cents: bet.potentialPayoutCents };
    case 'WON':
      return { label: 'Pago recibido', cents: bet.payoutCents };
    case 'VOID':
      return { label: 'Reembolsado', cents: bet.payoutCents };
    case 'LOST':
      return { label: 'Sin pago', cents: null };
  }
}
