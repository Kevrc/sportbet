import { ApiError } from '../../core/http/error.interceptor';

// Traduce los códigos del backend a mensajes para el usuario.
const MESSAGES: Record<string, string> = {
  INVALID_CREDENTIALS: 'El email o la contraseña no son correctos.',
  USER_BLOCKED: 'Tu cuenta está bloqueada. Contacta al administrador.',
  EMAIL_TAKEN: 'Ya existe una cuenta con este email.',
  VALIDATION_ERROR: 'Revisa los campos marcados.',
  NETWORK_ERROR: 'No pudimos conectar con el servidor. Revisa tu conexión e inténtalo de nuevo.',
};

export function authErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    return MESSAGES[err.code] ?? 'Ocurrió un error inesperado. Inténtalo de nuevo.';
  }
  return 'Ocurrió un error inesperado. Inténtalo de nuevo.';
}

/** Errores por campo que devolvió el backend (400 VALIDATION_ERROR o 409 EMAIL_TAKEN). */
export function authFieldErrors(err: unknown): Record<string, string> {
  if (!(err instanceof ApiError)) return {};
  if (err.code === 'EMAIL_TAKEN') return { email: MESSAGES['EMAIL_TAKEN'] };
  return err.fields ?? {};
}
