// Resolve API base URL:
// - Vite dev (npm run dev, Docker UI): always use relative /v1 so the dev-server proxy
//   (API_PROXY_TARGET → notification-service:8080 in Docker) is used. A baked
//   VITE_API_BASE_URL in ui/.env would otherwise send the browser to the wrong host
//   (e.g. localhost:8080 on the client) and hang /auth and ProtectedRoute.
// - Production builds: use VITE_API_BASE_URL when set (e.g. Vercel), else same-origin /v1.
const rawApiBaseUrl = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.trim() || '';
const normalizedApiBaseUrl = rawApiBaseUrl.replace(/\/+$/, '').replace(/\/v1$/i, '');
const isLocalUiHost =
  typeof window !== 'undefined' &&
  (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
const pointsToLoopbackApi = /^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/i.test(normalizedApiBaseUrl);
export const API_V1_BASE_URL = import.meta.env.DEV
  ? '/v1'
  : normalizedApiBaseUrl && !(isLocalUiHost && pointsToLoopbackApi)
    ? `${normalizedApiBaseUrl}/v1`
    : '/v1';

export const buildApiUrl = (path: string) => {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${API_V1_BASE_URL}${normalizedPath}`;
};

