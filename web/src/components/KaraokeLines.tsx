import type { LyricLine } from '../types';

type Variant = 'stacked' | 'overlay';

type Props = {
  lines: LyricLine[];
  currentMs: number;
  variant?: Variant;
  fullscreen?: boolean;
};

function findLineIndex(lines: LyricLine[], currentMs: number): number {
  if (lines.length === 0) return -1;
  if (currentMs < lines[0].startMs) return -1;
  let lo = 0;
  let hi = lines.length - 1;
  let ans = 0;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    if (lines[mid].startMs <= currentMs) {
      ans = mid;
      lo = mid + 1;
    } else {
      hi = mid - 1;
    }
  }
  return ans;
}

export function KaraokeLines({ lines, currentMs, variant = 'stacked', fullscreen = false }: Props) {
  const idx = findLineIndex(lines, currentMs);
  const active = idx >= 0 ? lines[idx] : null;
  const next = idx + 1 < lines.length ? lines[idx + 1] : null;

  const container =
    variant === 'overlay'
      ? fullscreen
        ? 'flex flex-col items-center gap-4 text-center'
        : 'flex flex-col items-center gap-2 text-center'
      : 'flex min-h-[50vh] flex-col items-center justify-center gap-6 px-4 text-center';

  const intro = variant === 'overlay' ? 'text-white/70' : 'text-mute';

  return (
    <div className={container}>
      {active ? (
        <>
          <LineRow line={active} currentMs={currentMs} active variant={variant} fullscreen={fullscreen} />
          {active.translation ? (
            <p
              className={
                variant === 'overlay'
                  ? `italic ${fullscreen ? 'text-[clamp(1rem,1.8vw,1.75rem)]' : 'text-[clamp(0.8rem,1.2vw,1.1rem)]'} text-white/60 [text-shadow:0_1px_3px_rgba(0,0,0,0.65)]`
                  : 'italic text-[clamp(0.9rem,2vw,1.3rem)] text-mute'
              }
            >
              {active.translation}
            </p>
          ) : null}
        </>
      ) : (
        <p className={`text-xs uppercase tracking-[0.18em] ${intro}`}>press play to begin</p>
      )}
      {next ? (
        <LineRow line={next} currentMs={currentMs} variant={variant} fullscreen={fullscreen} />
      ) : (
        <div className="h-[1em] w-full" aria-hidden />
      )}
    </div>
  );
}

function LineRow({
  line,
  currentMs,
  active = false,
  variant,
  fullscreen,
}: {
  line: LyricLine;
  currentMs: number;
  active?: boolean;
  variant: Variant;
  fullscreen: boolean;
}) {
  const sizeCls = sizeClass(variant, active, fullscreen);

  return (
    <p className={sizeCls}>
      {line.words.map((w, i) => {
        const state = !active
          ? 'upcoming'
          : currentMs >= w.endMs
            ? 'sung'
            : currentMs >= w.startMs
              ? 'singing'
              : 'upcoming';
        const cls = colorClass(state, active, variant);
        return (
          <span key={i} className={`${cls} transition-colors duration-100`}>
            {i > 0 ? ' ' : ''}
            {w.text}
          </span>
        );
      })}
    </p>
  );
}

function sizeClass(variant: Variant, active: boolean, fullscreen: boolean): string {
  if (variant === 'overlay') {
    if (fullscreen) {
      return active
        ? 'font-display font-semibold leading-tight tracking-tightest text-[clamp(2.5rem,5.5vw,5.5rem)]'
        : 'font-display font-medium leading-tight tracking-tightest text-[clamp(1.5rem,3vw,3rem)]';
    }
    return active
      ? 'font-display font-semibold leading-tight tracking-tightest text-[clamp(1.5rem,2.6vw,2.5rem)]'
      : 'font-display font-medium leading-tight tracking-tightest text-[clamp(1rem,1.6vw,1.5rem)]';
  }
  return active
    ? 'font-display text-[clamp(2rem,6vw,4.25rem)] font-semibold leading-tight tracking-tightest'
    : 'font-display text-[clamp(1.25rem,3.5vw,2.25rem)] font-medium leading-tight tracking-tightest';
}

const OVERLAY_BASE_SHADOW = '[text-shadow:0_1px_3px_rgba(0,0,0,0.65)]';
const OVERLAY_SINGING_SHADOW =
  '[text-shadow:-1px_-1px_0_#fff,1px_-1px_0_#fff,-1px_1px_0_#fff,1px_1px_0_#fff,0_2px_4px_rgba(0,0,0,0.55)]';

function colorClass(
  state: 'sung' | 'singing' | 'upcoming',
  active: boolean,
  variant: Variant,
): string {
  if (variant === 'overlay') {
    if (state === 'singing') return `text-accent ${OVERLAY_SINGING_SHADOW}`;
    if (state === 'sung') return `text-white ${OVERLAY_BASE_SHADOW}`;
    return `${active ? 'text-white/55' : 'text-white/45'} ${OVERLAY_BASE_SHADOW}`;
  }
  if (state === 'singing') return 'text-accent';
  if (state === 'sung') return 'text-ink';
  return active ? 'text-mute' : 'text-mute/70';
}
