import { useEffect, useRef, useState } from 'react';
import type { LyricLine } from '../types';
import { KaraokeLines } from './KaraokeLines';

type YTPlayer = {
  getCurrentTime: () => number;
  destroy: () => void;
};

type YTNamespace = {
  Player: new (
    el: HTMLElement,
    opts: {
      videoId: string;
      playerVars?: Record<string, number | string>;
      events?: Record<string, (e: unknown) => void>;
    },
  ) => YTPlayer;
};

declare global {
  interface Window {
    YT?: YTNamespace;
    onYouTubeIframeAPIReady?: () => void;
  }
}

const IFRAME_API_SRC = 'https://www.youtube.com/iframe_api';
let apiPromise: Promise<YTNamespace> | null = null;

function loadYouTubeApi(): Promise<YTNamespace> {
  if (window.YT?.Player) return Promise.resolve(window.YT);
  if (apiPromise) return apiPromise;

  apiPromise = new Promise<YTNamespace>((resolve) => {
    const prev = window.onYouTubeIframeAPIReady;
    window.onYouTubeIframeAPIReady = () => {
      prev?.();
      if (window.YT) resolve(window.YT);
    };
    if (!document.querySelector(`script[src="${IFRAME_API_SRC}"]`)) {
      const tag = document.createElement('script');
      tag.src = IFRAME_API_SRC;
      tag.async = true;
      document.head.appendChild(tag);
    }
  });
  return apiPromise;
}

type Props = {
  youtubeId: string;
  lines: LyricLine[];
};

export function KaraokePlayer({ youtubeId, lines }: Props) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mountRef = useRef<HTMLDivElement | null>(null);
  const playerRef = useRef<YTPlayer | null>(null);
  const rafRef = useRef<number | null>(null);
  const [currentMs, setCurrentMs] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);

  useEffect(() => {
    let cancelled = false;

    loadYouTubeApi().then((YT) => {
      if (cancelled || !mountRef.current) return;
      playerRef.current = new YT.Player(mountRef.current, {
        videoId: youtubeId,
        playerVars: {
          rel: 0,
          modestbranding: 1,
          playsinline: 1,
          fs: 0,
        },
      });
      tick();
    });

    const tick = () => {
      const p = playerRef.current;
      if (p) {
        try {
          const t = p.getCurrentTime();
          if (typeof t === 'number') setCurrentMs(Math.round(t * 1000));
        } catch {
          // player not ready yet
        }
      }
      rafRef.current = requestAnimationFrame(tick);
    };

    return () => {
      cancelled = true;
      if (rafRef.current != null) cancelAnimationFrame(rafRef.current);
      try {
        playerRef.current?.destroy();
      } catch {
        // best effort
      }
      playerRef.current = null;
    };
  }, [youtubeId]);

  useEffect(() => {
    const sync = () => setIsFullscreen(document.fullscreenElement === containerRef.current);
    document.addEventListener('fullscreenchange', sync);
    return () => document.removeEventListener('fullscreenchange', sync);
  }, []);

  const toggleFullscreen = async () => {
    const el = containerRef.current;
    if (!el) return;
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        await el.requestFullscreen();
      }
    } catch {
      // user denied or unsupported
    }
  };

  const overlayVisibility = isFullscreen ? 'block' : 'hidden lg:block';

  return (
    <div className="flex flex-col gap-10 lg:gap-0">
      <div
        ref={containerRef}
        className="relative aspect-video w-full overflow-hidden border border-line bg-black data-[fullscreen=true]:aspect-auto data-[fullscreen=true]:h-screen data-[fullscreen=true]:w-screen data-[fullscreen=true]:border-0"
        data-fullscreen={isFullscreen}
      >
        <div ref={mountRef} className="absolute inset-0 h-full w-full" />

        <button
          type="button"
          onClick={toggleFullscreen}
          aria-label={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
          className="absolute right-3 top-3 z-20 inline-flex h-9 items-center gap-2 bg-black/55 px-3 text-[10px] font-medium uppercase tracking-[0.18em] text-white/90 backdrop-blur-sm transition-colors hover:bg-black/80"
        >
          <FullscreenIcon isFullscreen={isFullscreen} />
          <span className="hidden sm:inline">{isFullscreen ? 'exit' : 'fullscreen'}</span>
        </button>

        <div
          className={`pointer-events-none absolute inset-x-0 bottom-0 z-10 bg-gradient-to-t from-black/85 via-black/55 to-transparent px-6 pb-6 pt-20 ${overlayVisibility}`}
        >
          <KaraokeLines
            lines={lines}
            currentMs={currentMs}
            variant="overlay"
            fullscreen={isFullscreen}
          />
        </div>
      </div>

      <div className={isFullscreen ? 'hidden' : 'lg:hidden'}>
        <KaraokeLines lines={lines} currentMs={currentMs} variant="stacked" />
      </div>
    </div>
  );
}

function FullscreenIcon({ isFullscreen }: { isFullscreen: boolean }) {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="square"
      aria-hidden
    >
      {isFullscreen ? (
        <>
          <path d="M5 1 V5 H1" />
          <path d="M9 1 V5 H13" />
          <path d="M5 13 V9 H1" />
          <path d="M9 13 V9 H13" />
        </>
      ) : (
        <>
          <path d="M1 5 V1 H5" />
          <path d="M13 5 V1 H9" />
          <path d="M1 9 V13 H5" />
          <path d="M13 9 V13 H9" />
        </>
      )}
    </svg>
  );
}
