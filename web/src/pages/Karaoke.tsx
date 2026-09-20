import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { api } from '../api';
import { DeleteVideoButton } from '../components/DeleteVideoButton';
import { KaraokePlayer } from '../components/KaraokePlayer';
import { PipelineProgress } from '../components/PipelineProgress';
import type { Video } from '../types';
import { useLive } from '../useLive';

export function KaraokePage() {
  const { id = '' } = useParams();
  const [fetched, setFetched] = useState<Video | null>(null);
  const [notFound, setNotFound] = useState(false);
  const live = useLive(fetched ? [fetched] : null);

  useEffect(() => {
    api
      .getVideo(id)
      .then((r) => setFetched(r.video))
      .catch(() => setNotFound(true));
  }, [id]);

  const video = live.videos.get(id) ?? fetched;
  const stage = live.stages.get(id);

  if (notFound) {
    return (
      <div className="container-page py-24 text-center text-sm uppercase tracking-[0.18em] text-mute">
        video not found · <Link to="/" className="underline hover:text-ink">back to library</Link>
      </div>
    );
  }
  if (!video) {
    return <p className="container-page py-24 text-center text-sm text-mute">loading…</p>;
  }

  // The fetched record keeps ownership info; live broadcasts don't carry it.
  const owned = fetched?.owned ?? false;

  return (
    <div className="container-page pb-24">
      <div className="hairline mb-6 flex items-center justify-between pt-6">
        <Link to="/" className="text-xs uppercase tracking-[0.18em] text-mute hover:text-ink">
          ← library
        </Link>
        <div className="flex items-center gap-4">
          <span className="text-xs uppercase tracking-[0.18em] text-mute">
            {video.language ?? '—'} · {video.status}
          </span>
          {owned ? <DeleteVideoButton videoId={video.id} variant="inline" redirectHome /> : null}
        </div>
      </div>

      <h1 className="mb-8 font-display text-3xl font-semibold leading-tight tracking-tightest sm:text-5xl">
        {video.title}
      </h1>

      {video.status === 'ready' && video.lyrics ? (
        <KaraokePlayer youtubeId={video.youtubeId} lines={video.lyrics} />
      ) : (
        <PipelineProgress stage={stage} status={video.status} errorMessage={video.errorMessage} />
      )}
    </div>
  );
}
