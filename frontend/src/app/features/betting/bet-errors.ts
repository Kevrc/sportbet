import { ApiError } from '../../core/http/error.interceptor';

export type BetFailureKind =
  | 'odds-changed'
  | 'market-closed'
  | 'insufficient-funds'
  | 'correlated'
  | 'validation'
  | 'conflict'
  | 'unauthorized'
  | 'network'
  | 'unknown';

export interface BetFailure {
  kind: BetFailureKind;
  message: string;
  /** Errores por campo del 400 (VALIDATION_ERROR). */
  fields?: Record<string, string>;
  /**
   * Si es true, reintentar con la misma Idempotency-Key es seguro: la apuesta pudo haberse
   * creado aunque no llegó la respuesta (red/5xx). En cualquier otro error se genera otra.
   */
  keepKey: boolean;
}

/** Traduce el error de POST /bets (códigos del contrato) a un aviso para el usuario. */
export function describeBetError(err: unknown): BetFailure {
  if (!(err instanceof ApiError)) {
    return { kind: 'unknown', message: 'No se pudo realizar la apuesta.', keepKey: false };
  }
  switch (err.code) {
    case 'ODDS_CHANGED':
      return {
        kind: 'odds-changed',
        message: 'Las cuotas cambiaron. Revisa las nuevas cuotas y confirma para apostar.',
        keepKey: false,
      };
    case 'MARKET_CLOSED':
      return {
        kind: 'market-closed',
        message: 'Uno de los mercados ya está cerrado. Quita esa selección para continuar.',
        keepKey: false,
      };
    case 'INSUFFICIENT_FUNDS':
      return {
        kind: 'insufficient-funds',
        message: 'Saldo insuficiente para esta apuesta.',
        keepKey: false,
      };
    case 'CORRELATED_SELECTIONS':
      return {
        kind: 'correlated',
        message: 'No puedes combinar dos selecciones del mismo partido.',
        keepKey: false,
      };
    case 'VALIDATION_ERROR':
      return {
        kind: 'validation',
        message: 'Revisa los datos de la apuesta.',
        fields: err.fields,
        keepKey: false,
      };
    case 'NETWORK_ERROR':
      return {
        kind: 'network',
        message:
          'No hay conexión con el servidor. Puedes reintentar sin riesgo de apostar dos veces.',
        keepKey: true,
      };
  }
  if (err.status === 401) {
    return {
      kind: 'unauthorized',
      message: 'Tu sesión venció. Inicia sesión de nuevo.',
      keepKey: false,
    };
  }
  if (err.status >= 500) {
    return {
      kind: 'network',
      message: 'El servidor no respondió bien. Puedes reintentar sin riesgo de apostar dos veces.',
      keepKey: true,
    };
  }
  if (err.status === 400) {
    return {
      kind: 'validation',
      message: err.message || 'Revisa los datos de la apuesta.',
      fields: err.fields,
      keepKey: false,
    };
  }
  if (err.status === 409) {
    return {
      kind: 'conflict',
      message: 'La apuesta cambió mientras la enviabas. Revisa tu boleto e inténtalo de nuevo.',
      keepKey: false,
    };
  }
  return {
    kind: 'unknown',
    message: err.message || 'No se pudo realizar la apuesta.',
    keepKey: false,
  };
}
