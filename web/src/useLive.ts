import { useEffect, useRef, useState } from 'react';
import type { Video } from './types';

type LiveEvent =
  | { type: 'video'; payload: Video }
  | { type: 'video_deleted'; payload: { id: string } }
  | { type: 'progress'; payload: { id: string; stage: string } };

export type Live = {
  /** Latest server state per video id, merged over the initial fetch. */
  videos: Map<string, Video>;
  /** Live pipeline stage per video id (only while processing). */
  stages: Map<string, string>;
  connected: boolean;
};

// One WebSocket for the whole app: every browser watches every video's
// progress in real time — this is the worker pool made visible.
export function useLive(initial: Video[] | null): Live {
  const [videos, setVideos] = useState<Map<string, Video>>(new Map());
  const [stages, setStages] = useState<Map<string, string>>(new Map());
  const [connected, setConnected] = useState(false);
  const retry = useRef<number | null>(null);

  useEffect(() => {
    if (!initial) return;
    setVideos((prev) => {
      const next = new Map(prev);
      for (const v of initial) if (!next.has(v.id)) next.set(v.id, v);
      return next;
    });
  }, [initial]);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let closed = false;

    const connect = () => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws';
      ws = new WebSocket(`${proto}://${location.host}/ws`);
      ws.onopen = () => setConnected(true);
      ws.onclose = () => {
        setConnected(false);
        if (!closed) retry.current = window.setTimeout(connect, 1500);
      };
      ws.onmessage = (e) => {
        const event = JSON.parse(e.data) as LiveEvent;
        if (event.type === 'video') {
          setVideos((prev) => new Map(prev).set(event.payload.id, event.payload));
          if (event.payload.status === 'ready') {
            setStages((prev) => {
              const next = new Map(prev);
              next.delete(event.payload.id);
              return next;
            });
          }
        } else if (event.type === 'video_deleted') {
          setVideos((prev) => {
            const next = new Map(prev);
            next.delete(event.payload.id);
            return next;
          });
        } else if (event.type === 'progress') {
          setStages((prev) => new Map(prev).set(event.payload.id, event.payload.stage));
        }
      };
    };
    connect();

    return () => {
      closed = true;
      if (retry.current) clearTimeout(retry.current);
      ws?.close();
    };
  }, []);

  return { videos, stages, connected };
}
