import { WalletTxType } from '../../core/api/models';
import { formatCents } from './money';
import { signedAmount, txLabel } from './wallet-format';

describe('txLabel', () => {
  it('tiene una etiqueta legible para cada tipo de movimiento', () => {
    const types: WalletTxType[] = [
      'DEPOSIT_INITIAL',
      'BET_STAKE',
      'BET_PAYOUT',
      'BET_REFUND',
      'BET_PAYOUT_REVERSAL',
      'BET_REFUND_REVERSAL',
      'ADMIN_ADJUSTMENT',
    ];
    for (const t of types) {
      expect(txLabel(t)).not.toBe(t);
    }
    expect(txLabel('BET_STAKE')).toBe('Apuesta');
    expect(txLabel('BET_PAYOUT')).toBe('Premio');
  });

  it('un tipo desconocido se muestra tal cual', () => {
    expect(txLabel('NUEVO' as WalletTxType)).toBe('NUEVO');
  });
});

describe('signedAmount', () => {
  it('pone "+" solo a los montos positivos', () => {
    expect(signedAmount(4210, formatCents)).toBe('+42.10');
    expect(signedAmount(-1000, formatCents)).toBe('-10.00');
    expect(signedAmount(0, formatCents)).toBe('0.00');
  });
});
