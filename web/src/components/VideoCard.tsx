import { Link } from 'react-router-dom';
import type { Video } from '../types';
import { DeleteVideoButton } from './DeleteVideoButton';

function fmtDuration(sec: number): string {
  if (!sec) return '—';
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

export function VideoCard({ video, stage }: { video: Video; stage?: string }) {
  const disabled = video.status !== 'ready';
  const statusText =
    video.status === 'ready' ? 'ready' : video.status === 'failed' ? 'failed' : (stage ?? 'processing…');

  const inner = (
    <article className="group flex h-full flex-col gap-3">
      <div className="relative aspect-video w-full overflow-hidden border border-line bg-line">
        <img
          src={video.thumbnailUrl}
          alt=""
          loading="lazy"
          className={`absolute inset-0 h-full w-full object-cover transition-transform duration-500 ${
            disabled ? 'opacity-60' : 'group-hover:scale-[1.02]'
          }`}
        />
        <span className="absolute bottom-2 right-2 bg-bg/90 px-2 py-0.5 text-[10px] uppercase tracking-[0.18em] text-ink">
          {fmtDuration(video.durationSec)}
        </span>
        {video.owned ? <DeleteVideoButton videoId={video.id} variant="card" /> : null}
      </div>
      <div className="flex items-start justify-between gap-3">
        <h3 className="line-clamp-2 text-base font-medium leading-snug text-ink">{video.title}</h3>
        <span className="shrink-0 text-[10px] uppercase tracking-[0.18em] text-mute">
          {video.language ?? '—'}
        </span>
      </div>
      <span
        className={`text-[10px] uppercase tracking-[0.18em] ${
          video.status === 'failed'
            ? 'text-accent'
            : video.status === 'processing'
              ? 'animate-pulse text-mute'
              : 'text-mute'
        }`}
      >
        {statusText}
      </span>
    </article>
  );

  if (disabled) return <div className="cursor-default">{inner}</div>;
  return (
    <Link to={`/karaoke/${video.id}`} className="block">
      {inner}
    </Link>
  );
}
