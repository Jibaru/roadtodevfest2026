# s1n.go — parallel karaoke from any YouTube link

A **Go + ADK** clone of [s1ng by Crafter Station](https://github.com/crafter-station/s1gn),
built for Google DevFest 2026. Paste YouTube URLs — several at once — and a
goroutine **worker pool** turns each one into a synced karaoke player with
word-level highlighting, streaming per-stage progress to every browser over
WebSocket.

Where the original used npm libraries and a GPT call, s1n.go uses three
**ADK Gemini agents**:

| Agent | Job | Replaces |
|---|---|---|
| Language Detective | lyrics language from title+description (covers/dubs aware) | gpt-4o-mini call |
| Romanizer | ja/ko → romaji, Latin words pass through | kuroshiro + hangul-romanization |
| Translator | per-line Spanish translation (English if the song is Spanish) | — new feature |

## Run it locally (no API key, no yt-dlp needed)

```bash
FAKE_AGENTS=1 make run          # embedded fake song + fake agents
```

Open http://localhost:8080. With the real pipeline:

```bash
make web                        # build the React SPA once
GEMINI_API_KEY=... make run     # needs yt-dlp on PATH (or YTDLP_PATH=...)
```

## Live

- **App**: https://s1ngo.crafter.run (VPS via Dokploy compose: Go binary + Postgres; pushes to `main` auto-redeploy)
- **Slides**: https://jibaru.github.io/roadtodevfest2026/ (GitHub Pages from `docs/`)

## How a video is processed

1. `POST /api/videos` validates the URL and returns instantly — the job goes
   into a buffered channel.
2. A worker (of `WORKERS`, default 3) picks it up. Two-phase yt-dlp fetch:
   manual subtitles in ko/ja/es/en first (never rate-limited), then the
   auto-caption in the detected source language only.
3. The Language Detective agent resolves the real lyrics language.
4. VTT is parsed; YouTube's "rolling" auto-sub cues are collapsed.
5. Romanizer (ja/ko) and Translator agents map the lines in batches.
6. Word-level timing is distributed by syllable weight; result lands in
   Postgres as `ready`. Every stage broadcasts over `/ws`.

Failures degrade, never crash: romanizer down → original script; translator
down → no translation line; YouTube bot-check → the card shows why and
resubmitting retries it.

### YouTube from a datacenter IP (the VPS reality)

| Scenario | Status on the VPS |
|---|---|
| Metadata + **manual** subtitle tracks | ✅ stable (with cookies) |
| **Auto-generated** captions | ⚠️ intermittent — retry by resubmitting |
| Anything from a residential IP (local run) | ✅ everything works |

Required setup for the VPS: `YTDLP_COOKIES_B64` (base64 of a Netscape
cookies.txt exported from a logged-in browser). `--ignore-no-formats-error`
is always passed (we never download media). `YTDLP_EXTRA_ARGS` exists for
experimenting with player clients. Show strategy: prefer songs with manual
subtitles (official K-pop/J-pop channels usually have them), pre-process
your setlist before going live, or run locally.

## Architecture

```
cmd/api                      wire everything
internal/video/domain        Video, LyricLine, VideoRepository, URL parsing
internal/pipeline            worker pool + per-video stages + broadcasts
internal/video/infra         memory | postgres (pgx) — same interface
internal/agents              ADK: detective + romanizer + translator (+ fakes)
internal/ytdlp               two-phase subtitle fetch, VTT parser, cue dedupe
internal/lines               syllable-weight word timing (faithful port)
internal/realtime            push-only WebSocket hub; slow clients dropped
web/                         React SPA (Vite), embedded via go:embed
docs/                        HTML slides (EN/ES) — GitHub Pages
```

Env: `PORT`, `GEMINI_API_KEY`, `FAKE_AGENTS`, `DATABASE_URL`, `WORKERS`,
`YTDLP_PATH`, `YTDLP_COOKIES`.

```bash
make test   # url/vtt/lines/pipeline tests, all offline
```

## Credits

UI and pipeline design ported from **[s1ng](https://github.com/crafter-station/s1gn)**
by [Crafter Station](https://crafterstation.com) — same team, different
runtime. Attribution intentional and visible, also in the app footer.
