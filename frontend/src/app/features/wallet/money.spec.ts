import { formatCents, parseToCents } from './money';

describe('formatCents', () => {
  it('formatea con dos decimales y separador de miles', () => {
    expect(formatCents(0)).toBe('0.00');
    expect(formatCents(5)).toBe('0.05');
    expect(formatCents(1050)).toBe('10.50');
    expect(formatCents(214500)).toBe('2 145.00');
    expect(formatCents(100_000)).toBe('1 000.00');
    expect(formatCents(1_000_000)).toBe('10 000.00');
  });

  it('pone el signo delante en los negativos', () => {
    expect(formatCents(-1050)).toBe('-10.50');
    expect(formatCents(-5)).toBe('-0.05');
  });
});

describe('parseToCents', () => {
  it('convierte con enteros, sin errores de coma flotante', () => {
    expect(parseToCents('10')).toBe(1000);
    expect(parseToCents('10.5')).toBe(1050);
    expect(parseToCents('10,50')).toBe(1050);
    expect(parseToCents('0.07')).toBe(7);
    expect(parseToCents('19.99')).toBe(1999);
    expect(parseToCents('  1.00 ')).toBe(100);
  });

  it('rechaza lo que no es un monto válido', () => {
    for (const bad of ['', ' ', 'abc', '-5', '1.234', '1.', '.5', '1e3', '1,2,3']) {
      expect(parseToCents(bad), bad).toBeNull();
    }
  });

  it('rechaza montos fuera del rango de enteros seguros', () => {
    expect(parseToCents('99999999999999999999')).toBeNull();
  });

  it('es la inversa de formatCents para montos sin separador', () => {
    expect(parseToCents(formatCents(4321).replace(' ', ''))).toBe(4321);
  });
});
