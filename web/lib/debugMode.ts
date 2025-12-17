export function isDebugModeEnabled(): boolean {
  if (process.env.NEXT_PUBLIC_DEBUG_UI === '1') {
    return true;
  }
  if (typeof window === 'undefined') {
    return false;
  }

  try {
    const params = new URLSearchParams(window.location.search);
    const flag = params.get('debug');
    if (flag === '1' || flag === 'true') {
      return true;
    }
  } catch {
    // ignore
  }

  try {
    return window.localStorage.getItem('alex_debug') === '1';
  } catch {
    return false;
  }
}

