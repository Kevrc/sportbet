import {
  betKind,
  betOutcome,
  betStatusLabel,
  marketName,
  selectionResultLabel,
} from './bet-format';

describe('bet-format', () => {
  it('traduce estados de apuesta y resultados de selección', () => {
    expect(betStatusLabel('PENDING')).toBe('Pendiente');
    expect(betStatusLabel('WON')).toBe('Ganada');
    expect(betStatusLabel('LOST')).toBe('Perdida');
    expect(betStatusLabel('VOID')).toBe('Anulada');
    expect(selectionResultLabel('WON')).toBe('Ganada');
    expect(selectionResultLabel('VOID')).toContain('1.00');
  });

  it('nombra los mercados', () => {
    expect(marketName('MATCH_RESULT')).toBe('Resultado final (1X2)');
    expect(marketName('TOTAL_GOALS_2_5')).toBe('Total de goles (2.5)');
    expect(marketName('BOTH_TEAMS_TO_SCORE')).toBe('Ambos equipos anotan');
  });

  it('distingue simple de combinada', () => {
    expect(betKind(1)).toBe('Simple');
    expect(betKind(2)).toBe('Combinada (2)');
    expect(betKind(5)).toBe('Combinada (5)');
  });

  describe('betOutcome', () => {
    const base = { potentialPayoutCents: 4210, payoutCents: 0 };

    it('pendiente: muestra el pago potencial', () => {
      expect(betOutcome({ ...base, status: 'PENDING' })).toEqual({
        label: 'Pago potencial',
        cents: 4210,
      });
    });

    it('ganada: muestra el pago recibido', () => {
      expect(betOutcome({ status: 'WON', potentialPayoutCents: 4210, payoutCents: 4210 })).toEqual({
        label: 'Pago recibido',
        cents: 4210,
      });
    });

    it('anulada: muestra el reembolso', () => {
      expect(betOutcome({ status: 'VOID', potentialPayoutCents: 4210, payoutCents: 1000 })).toEqual(
        {
          label: 'Reembolsado',
          cents: 1000,
        },
      );
    });

    it('perdida: no hay pago', () => {
      expect(betOutcome({ ...base, status: 'LOST' })).toEqual({ label: 'Sin pago', cents: null });
    });
  });
});
