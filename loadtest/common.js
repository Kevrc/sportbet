// Utilidades compartidas por los escenarios de carga.
import http from 'k6/http';
import { check, fail } from 'k6';

export const BASE = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const JSON_HEADERS = { 'Content-Type': 'application/json' };

export function auth(token) {
  return { headers: { ...JSON_HEADERS, Authorization: `Bearer ${token}` } };
}

/** Registra un usuario nuevo y devuelve su token de acceso. */
export function register(prefix, i) {
  const email = `${prefix}-${Date.now()}-${i}@carga.test`;
  const res = http.post(
    `${BASE}/auth/register`,
    JSON.stringify({ email, password: 'carga-segura-123', displayName: `Carga ${i}` }),
    { headers: JSON_HEADERS, tags: { name: 'registro' } },
  );
  if (!check(res, { 'registro 201': (r) => r.status === 201 })) {
    fail(`no se pudo registrar ${email}: ${res.status} ${res.body}`);
  }
  return res.json('accessToken');
}

/** Selecciones abiertas del 1X2 de los partidos programados, con su cuota actual. */
export function openSelections() {
  const events = http.get(`${BASE}/events?status=SCHEDULED`).json();
  const out = [];
  for (const e of events) {
    const detail = http.get(`${BASE}/events/${e.id}`).json();
    for (const m of detail.markets || []) {
      if (m.code !== 'MATCH_RESULT' || m.status !== 'OPEN') continue;
      for (const s of m.selections) out.push({ eventId: e.id, selectionId: s.id, oddX100: s.oddX100 });
    }
  }
  if (out.length === 0) fail('no hay mercados abiertos: corre loadtest/seed.sql y espera ~15 s');
  return out;
}

export function uuid() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16);
  });
}
