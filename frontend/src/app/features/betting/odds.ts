// Cuotas en centésimas (214 = 2.14). Replica las reglas de docs/adr/0001-decisiones-base.md.

export const MIN_STAKE_CENTS = 100;
export const MAX_STAKE_CENTS = 50_000;
export const MAX_TOTAL_ODD_X100 = 50_000; // 500.00
export const MAX_PAYOUT_CENTS = 1_000_000;
export const MAX_SELECTIONS = 5;

/** 214 → "2.14". */
export function formatOdd(oddX100: number): string {
  const n = Math.trunc(oddX100);
  return `${Math.floor(n / 100)}.${(n % 100).toString().padStart(2, '0')}`;
}

/** Cuota total: trunca a centésimas en cada multiplicación y la limita a 500.00. */
export function combineOdds(oddsX100: readonly number[]): number {
  let total = 100;
  for (const odd of oddsX100) {
    total = Math.floor((total * odd) / 100);
    if (total >= MAX_TOTAL_ODD_X100) return MAX_TOTAL_ODD_X100;
  }
  return total;
}

/** payout = floor(stake × cuota / 100). Sin aplicar el tope de pago (el boleto lo avisa). */
export function potentialPayoutCents(stakeCents: number, totalOddX100: number): number {
  return Math.floor((stakeCents * totalOddX100) / 100);
}
