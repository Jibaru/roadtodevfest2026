import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';

export function AddVideoForm() {
  const navigate = useNavigate();
  const [url, setUrl] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || pending) return;
    setPending(true);
    setError(null);
    try {
      const { video } = await api.createVideo(url.trim());
      setUrl('');
      navigate(`/karaoke/${video.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setPending(false);
    }
  };

  return (
    <form onSubmit={onSubmit} className="w-full">
      <div className="flex flex-col gap-3 sm:flex-row">
        <input
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          type="url"
          required
          placeholder="https://youtube.com/watch?v=…"
          className="h-14 flex-1 rounded-none border border-ink bg-bg px-4 text-base text-ink outline-none placeholder:text-mute focus:border-accent"
          autoComplete="off"
          inputMode="url"
        />
        <button
          type="submit"
          disabled={pending}
          className="h-14 whitespace-nowrap rounded-none border border-ink bg-ink px-6 text-sm font-medium uppercase tracking-[0.18em] text-bg transition-colors hover:bg-accent hover:border-accent disabled:cursor-progress disabled:opacity-60"
        >
          {pending ? 'queueing…' : 'make karaoke'}
        </button>
      </div>
      {error ? (
        <p className="mt-3 text-sm text-accent">{error}</p>
      ) : (
        <p className="mt-3 text-xs uppercase tracking-[0.18em] text-mute">
          queued instantly · a Go worker pool processes several videos in parallel
        </p>
      )}
    </form>
  );
}
