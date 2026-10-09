import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';

import { AuthStore } from '../../../core/auth/auth.store';
import { authErrorMessage, authFieldErrors } from '../auth-errors';
import { safeReturnUrl } from '../guest.guard';
import { MIN_PASSWORD_CHARS, passwordRules } from '../password-rules';

type Field = 'email' | 'displayName' | 'password';

@Component({
  selector: 'app-register-page',
  imports: [ReactiveFormsModule, RouterLink],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './register.page.html',
  styleUrl: './auth-form.scss',
})
export class RegisterPage {
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);

  readonly returnUrl = input<string>();

  protected readonly minChars = MIN_PASSWORD_CHARS;
  protected readonly submitting = signal(false);
  protected readonly error = signal<string | null>(null);
  /** Errores por campo que devolvió el servidor. */
  protected readonly serverErrors = signal<Record<string, string>>({});

  protected readonly form = inject(FormBuilder).nonNullable.group({
    email: ['', [Validators.required, Validators.email]],
    displayName: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(40)]],
    password: ['', [Validators.required, passwordRules]],
  });

  constructor() {
    // Al editar un campo se borra el error que dio el servidor para ese campo.
    for (const name of Object.keys(this.form.controls) as Field[]) {
      this.form.controls[name].valueChanges.subscribe(() => {
        if (this.serverErrors()[name]) {
          const { [name]: _, ...rest } = this.serverErrors();
          this.serverErrors.set(rest);
        }
      });
    }
  }

  protected fieldError(name: Field): string | null {
    const server = this.serverErrors()[name];
    if (server) return server;
    const c = this.form.controls[name];
    if (!c.invalid || !(c.touched || c.dirty)) return null;
    if (c.hasError('required')) return 'Este campo es obligatorio.';
    switch (name) {
      case 'email':
        return 'Escribe un email válido.';
      case 'displayName':
        return 'Debe tener entre 2 y 40 caracteres.';
      case 'password':
        return c.hasError('maxBytes')
          ? 'Es demasiado larga (máximo 72 bytes).'
          : `Debe tener al menos ${MIN_PASSWORD_CHARS} caracteres.`;
    }
  }

  protected submit(): void {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.submitting.set(true);
    this.error.set(null);
    this.serverErrors.set({});
    const { email, displayName, password } = this.form.getRawValue();
    this.auth.register({ email, displayName: displayName.trim(), password }).subscribe({
      next: () => this.router.navigateByUrl(safeReturnUrl(this.returnUrl())),
      error: (err) => {
        this.serverErrors.set(authFieldErrors(err));
        this.error.set(authErrorMessage(err));
        this.submitting.set(false);
      },
    });
  }
}
