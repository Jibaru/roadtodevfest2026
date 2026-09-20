import { fingerprint } from './fingerprint';
import type { Video } from './types';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-Fingerprint': fingerprint(),
      ...init?.headers,
    },
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  listVideos: () => request<{ videos: Video[] | null }>('/api/videos'),
  getVideo: (id: string) => request<{ video: Video }>(`/api/videos/${id}`),
  createVideo: (url: string) =>
    request<{ video: Video }>('/api/videos', { method: 'POST', body: JSON.stringify({ url }) }),
  deleteVideo: (id: string) => request<void>(`/api/videos/${id}`, { method: 'DELETE' }),
};
