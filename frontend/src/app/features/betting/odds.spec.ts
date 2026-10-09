import { combineOdds, formatOdd, MAX_TOTAL_ODD_X100, potentialPayoutCents } from './odds';

describe('formatOdd', () => {
  it('muestra la cuota con dos decimales', () => {
    expect(formatOdd(214)).toBe('2.14');
    expect(formatOdd(101)).toBe('1.01');
    expect(formatOdd(100)).toBe('1.00');
    expect(formatOdd(5000)).toBe('50.00');
    expect(formatOdd(250)).toBe('2.50');
  });
});

describe('combineOdds', () => {
  it('una sola selección conserva su cuota', () => {
    expect(combineOdds([214])).toBe(214);
  });

  it('multiplica y trunca a centésimas (2.14 × 1.97 = 4.2158 → 4.21)', () => {
    expect(combineOdds([214, 197])).toBe(421);
  });

  it('trunca en cada paso, no solo al final', () => {
    // 1.33 × 1.33 = 1.7689 → 1.76; × 1.33 = 2.3408 → 2.34 (sin truncar el paso intermedio: 2.35)
    expect(combineOdds([133, 133, 133])).toBe(234);
  });

  it('una selección VOID (1.00) no cambia la cuota', () => {
    expect(combineOdds([214, 100, 197])).toBe(421);
  });

  it('limita la cuota total a 500.00', () => {
    expect(combineOdds([5000, 5000])).toBe(MAX_TOTAL_ODD_X100);
    expect(combineOdds([1000, 1000, 1000, 1000, 1000])).toBe(MAX_TOTAL_ODD_X100);
  });

  it('sin selecciones es 1.00', () => {
    expect(combineOdds([])).toBe(100);
  });
});

describe('potentialPayoutCents', () => {
  it('ejemplo del ADR: 10.00 × 4.21 = 42.10', () => {
    expect(potentialPayoutCents(1000, combineOdds([214, 100, 197]))).toBe(4210);
  });

  it('redondea hacia abajo al centavo', () => {
    expect(potentialPayoutCents(101, 150)).toBe(151); // 151.5
  });

  it('con cuota 1.00 devuelve el stake', () => {
    expect(potentialPayoutCents(2500, 100)).toBe(2500);
  });
});
