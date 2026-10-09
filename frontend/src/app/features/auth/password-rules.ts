import { AbstractControl, ValidationErrors, ValidatorFn } from '@angular/forms';

// Las mismas reglas que el backend (auth/domain/password.go): mínimo 8
// caracteres y máximo 72 bytes, el límite de bcrypt.
export const MIN_PASSWORD_CHARS = 8;
export const MAX_PASSWORD_BYTES = 72;

const encoder = new TextEncoder();

export function charCount(value: string): number {
  return [...value].length;
}

export function byteLength(value: string): number {
  return encoder.encode(value).length;
}

export const passwordRules: ValidatorFn = (control: AbstractControl): ValidationErrors | null => {
  const value = String(control.value ?? '');
  if (!value) return null; // Validators.required se encarga
  if (charCount(value) < MIN_PASSWORD_CHARS) return { minChars: true };
  if (byteLength(value) > MAX_PASSWORD_BYTES) return { maxBytes: true };
  return null;
};
