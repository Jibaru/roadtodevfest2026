// Self-issued browser fingerprint, ported from the original: whoever
// added a video can delete it. The backend stores only a hash.
const STORAGE_KEY = 's1ngo-fingerprint';

export function fingerprint(): string {
  try {
    const existing = window.localStorage.getItem(STORAGE_KEY);
    if (existing && existing.length >= 8) return existing;
    const fresh = window.crypto.randomUUID();
    window.localStorage.setItem(STORAGE_KEY, fresh);
    return fresh;
  } catch {
    return 'anonymous-visitor';
  }
}
