export const environment = {
  // El proxy de ng serve envía /api al backend (proxy.conf.json) o a Prism (proxy.mock.json).
  apiUrl: '/api/v1',
  // Si el marcador en vivo tiene más de este tiempo, se muestra como desactualizado.
  liveStaleAfterMs: 5 * 60 * 1000,
};
