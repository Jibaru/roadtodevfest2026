# Agent Arena — Live AI Code Review

The audience proposes public GitHub repos, three AI reviewer agents tear into
the winner **in parallel** — choosing which files to read with a typed
`read_file` tool, live — and the audience votes the most valuable finding.
Built in **Go** with **[ADK Go v2](https://adk.dev)** and **Gemini**, for
Google DevFest 2026.

The crew: **Bug Hunter** (correctness), **Sentinel** (security) and
**The Simplifier** (readability), plus a **Lead Reviewer** who closes each
round with a verdict. The reviewers compete: every audience vote scores a
point on the leaderboard.

## Run it locally (no API key needed)

```bash
FAKE_AGENTS=1 PRESENTER_TOKEN=dev make run
```

- Audience: http://localhost:8080
- Stage (presenter controls): http://localhost:8080/stage?token=dev

`FAKE_AGENTS=1` runs the whole show offline with canned findings — use it for
rehearsals that don't burn tokens. With a real key:

```bash
GEMINI_API_KEY=... PRESENTER_TOKEN=dev make run
```

## Live URLs

- **Demo**: https://agent-arena.crafter.run (VPS via Dokploy; pushes to `main` auto-redeploy)
- **Slides**: https://jibaru.github.io/roadtodevfest2026/ (GitHub Pages from `docs/`)

## Deploy to Cloud Run (personal account, one command)

```bash
./scripts/setup.sh    # one-time: isolated 'devfest' gcloud config, personal login, .env
./scripts/deploy.sh   # every deploy: Cloud Run, prints audience + stage URLs
./scripts/teardown.sh # after the event
```

The setup wizard creates a **named gcloud configuration** (`devfest`) scoped
with `CLOUDSDK_ACTIVE_CONFIG_NAME` — your work gcloud config is never touched.
`--max-instances 1` is load-bearing: session state lives in memory.

## The show (60 min)

| Segment | Time |
|---|---|
| The agentic era + why Go | 8 min |
| ADK Go concepts | 5 min |
| **Live reviews with the audience** | 25 min |
| Architecture walkthrough | 10 min |
| Live-code the search_code tool | 5 min |
| Q&A | 7 min |

Slides: `docs/index.html` (arrows to navigate, `L` toggles EN/ES).

## Live-coding cheat sheet

The `search_code` tool (grep across the whole snapshot) already exists in
`internal/agents/searchcode.go`. On stage, wire it into `Crew.Review` in
`internal/agents/crew.go` (see the `LIVE-CODING MOMENT` comment), redeploy,
and the next round's reviewers can hunt patterns across every file at once.

## Failure playbook

| Failure | What happens |
|---|---|
| Gemini down mid-show | The broken reviewer ships a visible "Reviewer went offline" finding; the round completes |
| Repo private/unfetchable | The verdict says so; pick another repo and advance again |
| Lead reviewer fails | Canned closing line |
| Hosting trouble | `FAKE_AGENTS=1` local run + a tunnel, rehearsed |
| Audience page dies | Platform chat as fallback for proposals/votes |

## Architecture

```
cmd/api                      wire everything
internal/review/domain       session, findings, votes, repo-URL validation
internal/review/service      the show's state machine (auto reviewing→results)
internal/review/infra        memory | file-snapshot — same interface
internal/agents              ADK Go: 3 reviewers + lead, read_file tool
internal/repofetch           GitHub tarball → capped in-memory snapshot
internal/realtime            WebSocket hub; slow clients get dropped
internal/{handlers,server}   thin HTTP layer, DTOs, middleware
web/                         audience + stage pages, go:embed, EN/ES
docs/                        HTML slides (EN/ES) — served via GitHub Pages
```

State machine: `idle → repos_open → reviewing → results → repos_open → …`.
The presenter drives it with one button; `reviewing → results` fires
automatically when the crew finishes. The audience drives everything else.

```bash
make test   # domain, repos, and full state-machine tests (all offline)
```
