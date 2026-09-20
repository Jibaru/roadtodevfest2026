import type { VideoStatus } from '../types';

// Mirrors the backend pipeline stages (internal/pipeline). Each step
// matches the raw stage strings broadcast over the WebSocket.
const STEPS = [
  { key: 'queued', label: 'Queued', match: (s: string) => s === 'queued' || s === 'retrying' },
  { key: 'fetch', label: 'Fetching subtitles', match: (s: string) => s.startsWith('fetching') },
  { key: 'detect', label: 'Detecting language', match: (s: string) => s.startsWith('detecting') },
  { key: 'romanize', label: 'Romanizing', match: (s: string) => s.startsWith('romanizing') },
  { key: 'translate', label: 'Translating', match: (s: string) => s.startsWith('translating') },
  { key: 'build', label: 'Building karaoke lines', match: (s: string) => s.startsWith('building') },
];

export function stageIndex(stage: string | undefined): number {
  if (!stage) return 0;
  const i = STEPS.findIndex((step) => step.match(stage));
  return i >= 0 ? i : 0;
}

type Props = {
  stage?: string;
  status: VideoStatus;
  errorMessage?: string;
};

// Vertical animated stepper: done steps get a filled dot and a check,
// the active step pulses in the accent color, pending steps stay muted.
export function PipelineProgress({ stage, status, errorMessage }: Props) {
  const active = status === 'ready' ? STEPS.length : stageIndex(stage);
  const failed = status === 'failed';

  return (
    <div className="mx-auto max-w-md py-10">
      <p className="mb-8 text-center text-xs uppercase tracking-[0.18em] text-mute">
        {status === 'ready' ? 'done' : failed ? 'processing failed' : 'processing'}
      </p>
      <ol>
        {STEPS.map((step, i) => {
          const state =
            i < active ? 'done' : i === active ? (failed ? 'failed' : 'active') : 'pending';
          return (
            <li key={step.key} className="relative flex gap-4 pb-8 last:pb-0">
              {i < STEPS.length - 1 ? (
                <span
                  aria-hidden
                  className={`absolute left-[11px] top-6 h-full w-px transition-colors duration-500 ${
                    i < active ? 'bg-ink' : 'bg-line'
                  }`}
                />
              ) : null}
              <span
                className={`relative z-10 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border text-[11px] transition-all duration-300 ${
                  state === 'done'
                    ? 'border-ink bg-ink text-bg'
                    : state === 'active'
                      ? 'animate-pulse border-accent bg-accent text-bg'
                      : state === 'failed'
                        ? 'border-accent bg-bg text-accent'
                        : 'border-line bg-bg text-mute'
                }`}
              >
                {state === 'done' ? '✓' : state === 'failed' ? '✕' : i + 1}
              </span>
              <span className="min-w-0 pt-0.5">
                <span
                  className={`block text-sm uppercase tracking-[0.14em] transition-colors ${
                    state === 'active'
                      ? 'font-medium text-ink'
                      : state === 'done'
                        ? 'text-ink'
                        : state === 'failed'
                          ? 'font-medium text-accent'
                          : 'text-mute'
                  }`}
                >
                  {step.label}
                </span>
                {state === 'active' && stage ? (
                  <span className="mt-1 block font-mono text-xs text-mute">{stage}…</span>
                ) : null}
                {state === 'failed' && errorMessage ? (
                  <span className="mt-1 block text-xs text-accent">{errorMessage}</span>
                ) : null}
              </span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

// Compact horizontal version for library cards: six segments filling up.
export function PipelineDots({ stage, status }: { stage?: string; status: VideoStatus }) {
  const active = status === 'ready' ? STEPS.length : stageIndex(stage);
  const failed = status === 'failed';
  return (
    <span className="inline-flex items-center gap-1" aria-hidden>
      {STEPS.map((step, i) => (
        <span
          key={step.key}
          className={`h-1 w-4 transition-colors duration-500 ${
            failed && i === active
              ? 'bg-accent'
              : i < active
                ? 'bg-ink'
                : i === active
                  ? 'animate-pulse bg-accent'
                  : 'bg-line'
          }`}
        />
      ))}
    </span>
  );
}
