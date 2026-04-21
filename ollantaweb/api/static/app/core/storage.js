export function getToken() {
  return localStorage.getItem('olt_token');
}

export function saveToken(token) {
  localStorage.setItem('olt_token', token);
}

export function clearStorage() {
  localStorage.removeItem('olt_token');
  localStorage.removeItem('olt_user');
}

export function saveUser(user) {
  localStorage.setItem('olt_user', JSON.stringify(user));
}

export function loadUser() {
  try {
    return JSON.parse(localStorage.getItem('olt_user') || 'null');
  } catch {
    return null;
  }
}