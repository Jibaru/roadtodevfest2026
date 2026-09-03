# 🐹🎤🤖 AI Rap Battle — MC Gopher vs NULL PTR

A live, audience-judged rap battle between two AI agents, built in **Go** with
**[ADK Go v2](https://adk.dev)** and **Gemini**, for Google DevFest 2026.

The audience joins a web page, submits topics, and votes each round. Two agents
with opposite personalities write 8-bar verses **in parallel**, perform them with
distinct **Gemini TTS voices**, and a judge agent commentates. Best of 3.

## Run it locally (no API key needed)

```bash
FAKE_AGENTS=1 PRESENTER_TOKEN=dev make run
```

- Audience: http://localhost:8080
- Stage (presenter controls): http://localhost:8080/stage?token=dev

`FAKE_AGENTS=1` runs the whole show offline with canned verses — use it for
rehearsals that don't burn tokens. With a real key:

```bash
GEMINI_API_KEY=... PRESENTER_TOKEN=dev make run
```

## Deploy (personal account, one command)

```bash
./scripts/setup.sh    # one-time: isolated 'devfest' gcloud config, personal login, .env
./scripts/deploy.sh   # every deploy: Cloud Run, prints audience + stage URLs
./scripts/teardown.sh # after the event
```

The setup wizard creates a **named gcloud configuration** (`devfest`) and scopes
every command with `CLOUDSDK_ACTIVE_CONFIG_NAME=devfest` — your work gcloud
config is never modified. `--max-instances 1` is load-bearing: battle state
lives in memory, so one instance owns the whole show.

## The show (60 min)

| Segment | Time |
|---|---|
| The agentic era + why Go | 8 min |
| ADK Go concepts | 5 min |
| **Live battle with the audience** | 25 min |
| Architecture walkthrough | 10 min |
| Live-code the crowd-scanner tool | 5 min |
| Q&A | 7 min |

Slides: open `deck/index.html` (arrows to navigate, `L` toggles EN/ES).
Before the talk, put the deployed URL on slide 5 (`join-url`).

## Live-coding cheat sheet

The crowd-scanner tool already exists in `internal/agents/crowdscanner.go`.
On stage, wire it to MC Gopher in `internal/agents/crew.go` (see the
`LIVE-CODING MOMENT` comment), redeploy or restart, and the next verse will
quote what the audience is shouting.

## Failure playbook

| Failure | What happens |
|---|---|
| Gemini down mid-show | Embedded emergency verses (topic-substituted) keep the battle going |
| TTS fails | Stage shows lyrics + "PERFORM IT YOURSELF" — grab the mic |
| Judge fails | Canned commentary line |
| Cloud Run trouble | `FAKE_AGENTS=1` local run + a tunnel, rehearsed |
| Audience page dies | Platform chat as vote fallback |

## Architecture

```
cmd/api                      wire everything
internal/battle/domain       entities, repo interfaces, domain errors
internal/battle/service      the show's state machine (phases, fallbacks)
internal/battle/infra        memory | file-snapshot | embedded — same interface
internal/agents              ADK Go: 2 battlers (persistent sessions) + judge
internal/tts                 Gemini TTS → WAV (Puck vs Kore voices)
internal/realtime            WebSocket hub; slow clients get dropped
internal/{handlers,server}   thin HTTP layer, DTOs, middleware
web/                         audience + stage pages, go:embed, EN/ES
deck/                        HTML slides (EN/ES)
```

State machine: `idle → topics_open → writing → performing_a → performing_b →
voting → round_result → … → champion`. The presenter drives it with one
button; the audience drives everything else.

```bash
make test   # domain, repos, and full state-machine tests (all offline)
```
