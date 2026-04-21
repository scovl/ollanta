import { getToken } from './storage.js';

const API = '/api/v1';

let unauthorizedHandler = null;

export function setUnauthorizedHandler(handler) {
  unauthorizedHandler = handler;
}

export async function apiFetch(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json' };
  const token = getToken();
  if (token) headers.Authorization = 'Bearer ' + token;
  if (opts.headers) Object.assign(headers, opts.headers);

  const res = await fetch(API + path, { ...opts, headers });

  if (res.status === 401) {
    if (unauthorizedHandler) unauthorizedHandler();
    throw new Error('Session expired');
  }
  if (res.status === 204) return null;
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || res.statusText);
  return body;
}