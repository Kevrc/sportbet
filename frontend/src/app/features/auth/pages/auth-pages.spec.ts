import { Type } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { Observable, of, throwError } from 'rxjs';

import { AuthStore } from '../../../core/auth/auth.store';
import { ApiError } from '../../../core/http/error.interceptor';
import { LoginPage } from './login.page';
import { RegisterPage } from './register.page';

type Result = Observable<unknown>;

function setup<T>(component: Type<T>, result: () => Result) {
  const calls: unknown[] = [];
  const store = {
    login: (body: unknown) => (calls.push(body), result()),
    register: (body: unknown) => (calls.push(body), result()),
  };
  TestBed.configureTestingModule({
    imports: [component],
    providers: [provideRouter([]), { provide: AuthStore, useValue: store }],
  });
  const router = TestBed.inject(Router);
  const navigated: string[] = [];
  router.navigateByUrl = ((url: string) => (navigated.push(url), Promise.resolve(true))) as never;
  const fixture: ComponentFixture<T> = TestBed.createComponent(component);
  return { fixture, calls, navigated };
}

function type(el: HTMLElement, selector: string, value: string) {
  const input = el.querySelector<HTMLInputElement>(selector)!;
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

async function submit(fixture: ComponentFixture<unknown>) {
  (fixture.nativeElement as HTMLElement).querySelector('form')!.dispatchEvent(new Event('submit'));
  await fixture.whenStable();
}

describe('LoginPage', () => {
  it('inicia sesión y vuelve a returnUrl', async () => {
    const { fixture, calls, navigated } = setup(LoginPage, () => of({}));
    fixture.componentRef.setInput('returnUrl', '/wallet');
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    type(el, '#login-email', 'ana@mail.com');
    type(el, '#login-password', 'secreto123');
    await submit(fixture);
    expect(calls).toEqual([{ email: 'ana@mail.com', password: 'secreto123' }]);
    expect(navigated).toEqual(['/wallet']);
  });

  it('no envía si el formulario es inválido y marca los campos', async () => {
    const { fixture, calls } = setup(LoginPage, () => of({}));
    await fixture.whenStable();
    await submit(fixture);
    const el = fixture.nativeElement as HTMLElement;
    expect(calls).toEqual([]);
    expect(el.querySelector('#login-email')!.getAttribute('aria-invalid')).toBe('true');
  });

  it('muestra el mensaje de credenciales inválidas', async () => {
    const { fixture, navigated } = setup(LoginPage, () =>
      throwError(() => new ApiError(401, 'INVALID_CREDENTIALS', 'x')),
    );
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    type(el, '#login-email', 'ana@mail.com');
    type(el, '#login-password', 'mala');
    await submit(fixture);
    expect(el.querySelector('[role=alert]')!.textContent).toContain('no son correctos');
    expect(el.querySelector('button[type=submit]')!.hasAttribute('disabled')).toBe(false);
    expect(navigated).toEqual([]);
  });
});

describe('RegisterPage', () => {
  async function fill(fixture: ComponentFixture<unknown>, password = 'secreto123') {
    await fixture.whenStable();
    const el = fixture.nativeElement as HTMLElement;
    type(el, '#register-email', 'ana@mail.com');
    type(el, '#register-name', '  Ana  ');
    type(el, '#register-password', password);
    return el;
  }

  it('crea la cuenta y va a eventos', async () => {
    const { fixture, calls, navigated } = setup(RegisterPage, () => of({}));
    await fill(fixture);
    await submit(fixture);
    expect(calls).toEqual([{ email: 'ana@mail.com', displayName: 'Ana', password: 'secreto123' }]);
    expect(navigated).toEqual(['/events']);
  });

  it('valida la contraseña en caracteres antes de enviar', async () => {
    const { fixture, calls } = setup(RegisterPage, () => of({}));
    const el = await fill(fixture, 'ññññ');
    await submit(fixture);
    expect(calls).toEqual([]);
    expect(el.textContent).toContain('al menos 8 caracteres');
  });

  it('marca el email cuando ya existe y limpia el error al editarlo', async () => {
    const { fixture } = setup(RegisterPage, () =>
      throwError(() => new ApiError(409, 'EMAIL_TAKEN', 'x')),
    );
    const el = await fill(fixture);
    await submit(fixture);
    expect(el.querySelector('#register-email-error')!.textContent).toContain('Ya existe');
    expect(el.querySelector('#register-email')!.getAttribute('aria-invalid')).toBe('true');

    type(el, '#register-email', 'otra@mail.com');
    await fixture.whenStable();
    expect(el.querySelector('#register-email-error')).toBeNull();
  });
});
