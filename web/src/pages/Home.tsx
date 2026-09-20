import { useEffect, useMemo, useState } from 'react';
import { api } from '../api';
import { AddVideoForm } from '../components/AddVideoForm';
import { VideoCard } from '../components/VideoCard';
import type { Video } from '../types';
import { useLive } from '../useLive';

export function HomePage() {
  const [initial, setInitial] = useState<Video[] | null>(null);
  const live = useLive(initial);

  useEffect(() => {
    api
      .listVideos()
      .then((r) => setInitial(r.videos ?? []))
      .catch(() => setInitial([]));
  }, []);

  const videos = useMemo(() => {
    const all = [...live.videos.values()];
    all.sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1));
    return all;
  }, [live.videos]);

  const processing = videos.filter((v) => v.status === 'processing').length;

  return (
    <div className="container-page">
      <section className="pt-12 pb-16 sm:pt-20 sm:pb-24">
        <p className="mb-6 text-xs uppercase tracking-[0.18em] text-mute">
          paste youtube urls → get karaoke, several at a time
        </p>
        <h1 className="font-display text-5xl font-semibold leading-[0.95] tracking-tightest sm:text-7xl">
          sing along.
          <br />
          <span className="text-mute">in parallel.</span>
        </h1>
        <p className="mt-6 max-w-xl text-base text-mute">
          s1n.go fetches subtitles with word-level timing and shows you two lines at a time. A Go
          worker pool processes every submission concurrently while Gemini agents detect the
          language, romanize Japanese &amp; Korean, and translate each line.
        </p>
        <div className="mt-10 max-w-3xl">
          <AddVideoForm />
        </div>
      </section>

      <section className="pb-24">
        <div className="hairline mb-8 flex items-baseline justify-between pt-6">
          <h2 className="text-xs uppercase tracking-[0.18em] text-mute">library</h2>
          <span className="text-xs uppercase tracking-[0.18em] text-mute">
            {videos.length} {videos.length === 1 ? 'video' : 'videos'}
            {processing > 0 ? ` · ${processing} processing` : ''}
          </span>
        </div>
        {initial === null ? (
          <p className="py-16 text-center text-sm text-mute">loading…</p>
        ) : videos.length === 0 ? (
          <p className="py-16 text-center text-sm text-mute">
            no videos yet. paste a url above to get started.
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-x-6 gap-y-10 sm:grid-cols-2 lg:grid-cols-3">
            {videos.map((v) => (
              <VideoCard key={v.id} video={v} stage={live.stages.get(v.id)} />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
