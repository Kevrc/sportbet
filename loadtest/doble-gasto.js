// Riesgo del plan: "el código debita saldo en dos pasos y permite doble gasto".
// Un usuario deja su saldo en 3.00 y lanza 20 apuestas simultáneas de 2.00:
// el débito atómico (UPDATE ... WHERE balance >= monto) debe aceptar solo una.
import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { BASE, auth, register, openSelections, uuid } from './common.js';

const accepted = new Counter('apuestas_aceptadas');
const rejected = new Counter('saldo_insuficiente');

export const options = {
  scenarios: {
    simultaneas: { executor: 'per-vu-iterations', vus: 20, iterations: 1, maxDuration: '30s' },
  },
  thresholds: {
    apuestas_aceptadas: ['count==1'],
    saldo_insuficiente: ['count==19'],
  },
};

function bet(token, pick, stakeCents) {
  return http.post(
    `${BASE}/bets`,
    JSON.stringify({ stakeCents, selections: [{ selectionId: pick.selectionId, oddX100: pick.oddX100 }] }),
    { headers: { ...auth(token).headers, 'Idempotency-Key': uuid() } },
  );
}

export function setup() {
  const token = register('doble-gasto', 0);
  const pick = openSelections()[0];
  // 1 000.00 de saldo inicial − 500.00 − 497.00 = 3.00
  for (const stake of [50000, 49700]) {
    check(bet(token, pick, stake), { 'apuesta para bajar el saldo 201': (r) => r.status === 201 });
  }
  const balance = http.get(`${BASE}/wallet`, auth(token)).json('balanceCents');
  check(balance, { 'saldo inicial de la prueba = 3.00': (b) => b === 300 });
  return { token, pick };
}

export default function (data) {
  const res = bet(data.token, data.pick, 200);
  if (res.status === 201) accepted.add(1);
  else if (res.status === 422 && res.json('error.code') === 'INSUFFICIENT_FUNDS') rejected.add(1);
}

export function teardown(data) {
  const balance = http.get(`${BASE}/wallet`, auth(data.token)).json('balanceCents');
  check(balance, { 'saldo final = 1.00 (nunca negativo)': (b) => b === 100 });
}
