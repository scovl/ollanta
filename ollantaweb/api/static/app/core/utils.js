export function fmtDate(d) {
  if (!d) return '\u2014';
  const date = new Date(d);
  const diff = Date.now() - date.getTime();
  if (diff < 60_000) return 'just now';
  if (diff < 3_600_000) return Math.floor(diff / 60_000) + 'm ago';
  if (diff < 86_400_000) return Math.floor(diff / 3_600_000) + 'h ago';
  if (diff < 604_800_000) return Math.floor(diff / 86_400_000) + 'd ago';
  return date.toLocaleDateString();
}

export function fmtNum(n) {
  return (n == null ? 0 : Number(n)).toLocaleString();
}

export function fmtK(n) {
  let value = n;
  if (value == null) value = 0;
  if (value >= 1_000_000) return (value / 1_000_000).toFixed(1) + 'M';
  if (value >= 1_000) return (value / 1_000).toFixed(1) + 'k';
  return String(value);
}

export function fmtPct(n) {
  if (n == null) return '\u2014';
  return Number(n).toFixed(1) + '%';
}

export function badgeClassForGateStatus(status) {
  if (status === 'OK') return 'badge-ok';
  if (status === 'WARN') return 'badge-warn';
  return 'badge-error';
}

export function cardClassForGateStatus(status) {
  if (status === 'OK') return 'card-gate-ok';
  if (status === 'WARN') return 'card-gate-warn';
  if (status === 'ERROR') return 'card-gate-error';
  return '';
}

export function escHtml(s) {
  if (s == null) return '';
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

export function escAttr(s) {
  return escHtml(s);
}