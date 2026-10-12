// Escenario PASA de carga (Persona 1).
// 50 usuarios simultáneos durante 1 minuto, con pausas de ~1 s entre acciones:
//   70 % navega (lista de eventos y detalle de un partido),
//   20 % consulta su billetera y su historial,
//   10 % apuesta (simple, 1.00 de stake).
// Requisitos no funcionales que verifica (thresholds):
//   p95 de lecturas < 300 ms, p95 de apostar < 500 ms, errores < 1 %.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE, auth, register, openSelections, uuid } from './common.js';

const USERS = Number(__ENV.USERS || 50);

export const options = {
  scenarios: {
    carga: { executor: 'constant-vus', vus: USERS, duration: __ENV.DURATION || '1m' },
  },
  thresholds: {
    'http_req_duration{tipo:leer}': ['p(95)<300'],
    'http_req_duration{tipo:apostar}': ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
  },
};

export function setup() {
  const tokens = [];
  for (let i = 0; i < USERS; i++) tokens.push(register('carga', i));
  return { tokens, selections: openSelections() };
}

export default function (data) {
  const token = data.tokens[(__VU - 1) % data.tokens.length];
  const roll = Math.random();
  if (roll < 0.7) {
    const leer = { tags: { tipo: 'leer', name: 'GET /events' } };
    const list = http.get(`${BASE}/events`, leer);
    check(list, { 'eventos 200': (r) => r.status === 200 });
    const pick = data.selections[Math.floor(Math.random() * data.selections.length)];
    const detail = http.get(`${BASE}/events/${pick.eventId}`, {
      tags: { tipo: 'leer', name: 'GET /events/{id}' },
    });
    check(detail, { 'detalle 200': (r) => r.status === 200 });
  } else if (roll < 0.9) {
    const opts = (name) => ({ ...auth(token), tags: { tipo: 'leer', name } });
    check(http.get(`${BASE}/wallet`, opts('GET /wallet')), { 'billetera 200': (r) => r.status === 200 });
    check(http.get(`${BASE}/bets`, opts('GET /bets')), { 'historial 200': (r) => r.status === 200 });
  } else {
    const pick = data.selections[Math.floor(Math.random() * data.selections.length)];
    const res = http.post(
      `${BASE}/bets`,
      JSON.stringify({ stakeCents: 100, selections: [{ selectionId: pick.selectionId, oddX100: pick.oddX100 }] }),
      {
        headers: { ...auth(token).headers, 'Idempotency-Key': uuid() },
        tags: { tipo: 'apostar', name: 'POST /bets' },
      },
    );
    check(res, { 'apuesta 201': (r) => r.status === 201 });
  }
  sleep(0.5 + Math.random());
}
