import { ApiError } from '../../core/http/error.interceptor';
import { describeBetError } from './bet-errors';

const err = (status: number, code: string, fields?: Record<string, string>) =>
  new ApiError(status, code, `mensaje ${code}`, fields);

describe('describeBetError', () => {
  it('ODDS_CHANGED pide confirmar las cuotas nuevas y no reutiliza la clave', () => {
    const f = describeBetError(err(409, 'ODDS_CHANGED'));
    expect(f.kind).toBe('odds-changed');
    expect(f.message).toContain('cuotas cambiaron');
    expect(f.keepKey).toBe(false);
  });

  it('mapea los códigos del contrato', () => {
    expect(describeBetError(err(422, 'MARKET_CLOSED')).kind).toBe('market-closed');
    expect(describeBetError(err(422, 'INSUFFICIENT_FUNDS')).kind).toBe('insufficient-funds');
    expect(describeBetError(err(422, 'CORRELATED_SELECTIONS')).kind).toBe('correlated');
  });

  it('VALIDATION_ERROR y 400 conservan los errores por campo', () => {
    const fields = { stakeCents: 'debe ser mayor a 100' };
    expect(describeBetError(err(400, 'VALIDATION_ERROR', fields))).toMatchObject({
      kind: 'validation',
      fields,
    });
    expect(describeBetError(err(400, 'BAD_JSON', fields)).fields).toEqual(fields);
  });

  it('un error de red o 5xx permite reintentar con la misma clave', () => {
    const net = describeBetError(err(0, 'NETWORK_ERROR'));
    expect(net).toMatchObject({ kind: 'network', keepKey: true });
    const srv = describeBetError(err(503, 'HTTP_503'));
    expect(srv).toMatchObject({ kind: 'network', keepKey: true });
  });

  it('401 pide iniciar sesión de nuevo', () => {
    expect(describeBetError(err(401, 'INVALID_TOKEN')).kind).toBe('unauthorized');
  });

  it('otro 409 (cuerpo distinto con la misma clave) se trata como conflicto', () => {
    const f = describeBetError(err(409, 'IDEMPOTENCY_CONFLICT'));
    expect(f).toMatchObject({ kind: 'conflict', keepKey: false });
  });

  it('un código desconocido muestra el mensaje del servidor', () => {
    const f = describeBetError(err(422, 'ALGO_NUEVO'));
    expect(f).toMatchObject({ kind: 'unknown', message: 'mensaje ALGO_NUEVO', keepKey: false });
  });

  it('un error que no es ApiError da un mensaje genérico', () => {
    expect(describeBetError(new Error('x'))).toMatchObject({ kind: 'unknown', keepKey: false });
    expect(describeBetError(null).message).toContain('No se pudo realizar');
  });
});
