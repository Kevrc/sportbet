// Dinero en centavos enteros (int64 en el backend). Nunca se usa float.

/** 214500 → "2 145.00". Los negativos llevan "-" delante. */
export function formatCents(cents: number): string {
  const sign = cents < 0 ? '-' : '';
  const abs = Math.abs(Math.trunc(cents));
  const whole = Math.floor(abs / 100)
    .toString()
    .replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
  const frac = (abs % 100).toString().padStart(2, '0');
  return `${sign}${whole}.${frac}`;
}

/**
 * "10.5" o "10,50" → 1050. Devuelve null si no es un monto válido
 * (vacío, negativo, letras o más de 2 decimales).
 */
export function parseToCents(input: string): number | null {
  const m = /^(\d+)(?:[.,](\d{1,2}))?$/.exec(input.trim());
  if (!m) return null;
  const cents = Number(m[1]) * 100 + Number((m[2] ?? '').padEnd(2, '0'));
  return Number.isSafeInteger(cents) ? cents : null;
}
