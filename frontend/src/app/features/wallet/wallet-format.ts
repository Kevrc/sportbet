import { WalletTxType } from '../../core/api/models';

const LABELS: Record<WalletTxType, string> = {
  DEPOSIT_INITIAL: 'Saldo inicial',
  BET_STAKE: 'Apuesta',
  BET_PAYOUT: 'Premio',
  BET_REFUND: 'Reembolso',
  BET_PAYOUT_REVERSAL: 'Premio revertido',
  BET_REFUND_REVERSAL: 'Reembolso revertido',
  ADMIN_ADJUSTMENT: 'Ajuste del administrador',
};

export function txLabel(type: WalletTxType): string {
  return LABELS[type] ?? type;
}

/** El signo negativo lo pone el formato; solo los montos positivos llevan "+". */
export function signedAmount(amountCents: number, format: (c: number) => string): string {
  return amountCents > 0 ? `+${format(amountCents)}` : format(amountCents);
}
