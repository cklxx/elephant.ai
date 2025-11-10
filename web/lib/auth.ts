const TOKEN_KEY = 'alex_auth_token';

export function getAuthToken(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  try {
    return localStorage.getItem(TOKEN_KEY) ?? '';
  } catch (error) {
    console.warn('[auth] Failed to read token from localStorage', error);
    return '';
  }
}

export function setAuthToken(token: string): void {
  if (typeof window === 'undefined') {
    return;
  }
  try {
    localStorage.setItem(TOKEN_KEY, token);
    document.cookie = `alex_auth_token=${encodeURIComponent(token)}; path=/; max-age=${7 * 24 * 60 * 60}`;
  } catch (error) {
    console.warn('[auth] Failed to persist token', error);
  }
}

export function clearAuthToken(): void {
  if (typeof window === 'undefined') {
    return;
  }
  try {
    localStorage.removeItem(TOKEN_KEY);
    document.cookie = 'alex_auth_token=; path=/; max-age=0';
  } catch (error) {
    console.warn('[auth] Failed to clear token', error);
  }
}
