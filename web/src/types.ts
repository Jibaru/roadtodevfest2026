export type VideoStatus = 'processing' | 'ready' | 'failed';

export type LyricWord = {
  text: string;
  startMs: number;
  endMs: number;
};

export type LyricLine = {
  startMs: number;
  endMs: number;
  words: LyricWord[];
  translation?: string;
};

export type Video = {
  id: string;
  youtubeId: string;
  title: string;
  thumbnailUrl: string;
  durationSec: number;
  language?: string;
  status: VideoStatus;
  errorMessage?: string;
  lyrics?: LyricLine[];
  owned?: boolean;
  createdAt: string;
};
