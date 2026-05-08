/**
 * Browser-based API helper for tests.
 * Runs inside the page so it can read the Bearer token from localStorage.
 */
export async function apiCall(
  page: import('@playwright/test').Page,
  method: string,
  path: string,
  body?: unknown,
): Promise<{ status: number; data: unknown }> {
  return page.evaluate(
    async ([m, p, b]: [string, string, unknown]) => {
      const token = localStorage.getItem('olt_token');
      const res = await fetch(`/api/v1${p}`, {
        method: m,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: b ? JSON.stringify(b) : undefined,
      });
      const data = await res.json().catch(() => null);
      return { status: res.status, data };
    },
    [method, path, body] as [string, string, unknown],
  );
}
