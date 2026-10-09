import { FormControl } from '@angular/forms';

import { ApiError } from '../../core/http/error.interceptor';
import { authErrorMessage, authFieldErrors } from './auth-errors';
import { safeReturnUrl } from './guest.guard';
import { byteLength, charCount, passwordRules } from './password-rules';

describe('passwordRules', () => {
  const check = (v: string) => passwordRules(new FormControl(v));

  it('cuenta caracteres, no bytes, para el mínimo', () => {
    expect(charCount('ññññ')).toBe(4);
    expect(byteLength('ññññ')).toBe(8);
    expect(check('ññññ')).toEqual({ minChars: true });
    expect(check('ññññññññ')).toBeNull();
  });

  it('rechaza más de 72 bytes, como bcrypt', () => {
    expect(check('a'.repeat(72))).toBeNull();
    expect(check('a'.repeat(73))).toEqual({ maxBytes: true });
    expect(check('ñ'.repeat(40))).toEqual({ maxBytes: true });
  });

  it('deja el vacío a Validators.required', () => {
    expect(check('')).toBeNull();
  });
});

describe('safeReturnUrl', () => {
  it('acepta rutas internas', () => {
    expect(safeReturnUrl('/wallet')).toBe('/wallet');
    expect(safeReturnUrl('/bets?status=PENDING')).toBe('/bets?status=PENDING');
  });

  it('evita redirecciones abiertas y bucles', () => {
    for (const bad of [undefined, '', 'https://evil.com', '//evil.com', '/auth/login']) {
      expect(safeReturnUrl(bad)).toBe('/events');
    }
  });
});

describe('auth-errors', () => {
  it('traduce códigos conocidos y desconocidos', () => {
    expect(authErrorMessage(new ApiError(401, 'INVALID_CREDENTIALS', 'x'))).toContain(
      'no son correctos',
    );
    expect(authErrorMessage(new ApiError(500, 'INTERNAL', 'x'))).toContain('inesperado');
    expect(authErrorMessage(new Error('x'))).toContain('inesperado');
  });

  it('asigna EMAIL_TAKEN al campo email y pasa los fields del backend', () => {
    expect(authFieldErrors(new ApiError(409, 'EMAIL_TAKEN', 'x'))).toEqual({
      email: 'Ya existe una cuenta con este email.',
    });
    expect(
      authFieldErrors(new ApiError(400, 'VALIDATION_ERROR', 'x', { password: 'corta' })),
    ).toEqual({
      password: 'corta',
    });
  });
});
