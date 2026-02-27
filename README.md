# cllama-passthrough

An LLM proxy that sits between your AI agents and their model providers. It holds the real API keys so the agents don't have to.

```
┌─────────┐         ┌──────────────┐         ┌──────────────┐
│  agent   │──req──▶ │    cllama    │──req──▶ │   provider   │
│          │◀─resp── │  passthrough │◀─resp── │  (OpenAI,    │
│ (no real │         │              │         │  Anthropic,  │
│  keys)   │  Bearer │ swaps token  │  real   │  OpenRouter, │
│          │  token  │ for real key │  key    │  Ollama)     │
└─────────┘         └──────────────┘         └──────────────┘
                           │
                     ┌─────┴─────┐
                     │  :8081    │
                     │  operator │
                     │  dashboard│
                     └───────────┘
```

## What problem does this solve?

If you're running AI agents on your homelab — bots that talk to LLMs, run on schedules, coordinate with each other — you have two problems:

1. **Keys everywhere.** Every agent needs an API key. If you run three bots, that's three places your `ANTHROPIC_API_KEY` lives. One misconfigured container and your key leaks.

2. **Who's spending what?** When three agents share an OpenRouter account, your bill is one number. You can't tell which agent burned $4.50 on Sonnet calls and which one spent $0.02 on Haiku.

`cllama-passthrough` solves both. It's one process that holds all the real keys, hands each agent a unique dummy token, and tracks every request by agent, model, and cost.

This is **credential starvation** — the agent literally cannot call the LLM directly because it doesn't have the credentials. All inference must pass through the proxy.

## How it works

1. Your agent sends a request to `http://cllama-passthrough:8080/v1/chat/completions` with `Authorization: Bearer tiverton:abc123...` and `"model": "anthropic/claude-sonnet-4"`.
2. The proxy parses the bearer token, validates the secret against the agent's `metadata.json`, and identifies the caller.
3. It splits the model on `/` — `anthropic` is the provider, `claude-sonnet-4` is the upstream model name.
4. It swaps the dummy token for the real `ANTHROPIC_API_KEY` and forwards the request.
5. The response streams back transparently. The agent never knows the proxy exists.
6. Token usage is extracted from the response, multiplied by the pricing table, and recorded per agent.

The proxy is OpenAI-compatible. Any tool that can talk to the OpenAI API works — just point its base URL at the proxy.

## Quick start

### Build and run locally

```bash
go build -o cllama-passthrough ./cmd/cllama-passthrough
./cllama-passthrough
```

API on `:8080`, dashboard on `:8081`. Open http://localhost:8081 to configure providers.

### Docker

```bash
docker build -t cllama-passthrough .
docker run -p 8080:8080 -p 8081:8081 cllama-passthrough
```

The image is ~15 MB (distroless, single static binary, no runtime dependencies).

### Try the spike demo

A self-contained demo that stands up a mock LLM backend, creates three trading-desk agents, fires a burst of requests, and leaves the dashboard running so you can poke around:

```bash
go test -tags spike -v -run TestSpikeLiveDashboard ./cmd/cllama-passthrough/...
```

Then open http://127.0.0.1:9081/ — you'll see providers, pod members, and cost breakdowns across three agents using different models and providers.

Press Ctrl-C to stop.

## Configuration

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | API server bind address |
| `UI_ADDR` | `:8081` | Operator dashboard bind address |
| `CLAW_CONTEXT_ROOT` | `/claw/context` | Directory containing per-agent context |
| `CLAW_AUTH_DIR` | `/claw/auth` | Directory for `providers.json` |
| `CLAW_POD` | *(empty)* | Pod name (shown in dashboard) |
| `OPENAI_API_KEY` | | Override for OpenAI provider key |
| `ANTHROPIC_API_KEY` | | Override for Anthropic provider key |
| `OPENROUTER_API_KEY` | | Override for OpenRouter provider key |

Provider keys can be set via env vars or via the web UI at `:8081`. Env vars take precedence.

### Agent context directory

Each agent needs a subdirectory under `CLAW_CONTEXT_ROOT`:

```
/claw/context/
├── tiverton/
│   ├── metadata.json    # {"token":"tiverton:abc123...","pod":"trading-desk","service":"tiverton","type":"openclaw"}
│   ├── AGENTS.md        # behavioral contract
│   └── CLAWDAPUS.md     # infrastructure map
├── westin/
│   └── ...
└── allen/
    └── ...
```

The `token` field in `metadata.json` is what the agent sends as its bearer token. The proxy validates requests against it using constant-time comparison.

When orchestrated by [Clawdapus](https://github.com/mostlydev/clawdapus), all of this is generated automatically by `claw compose up`.

### Provider registry (`providers.json`)

```json
{
  "providers": {
    "anthropic": {
      "base_url": "https://api.anthropic.com/v1",
      "api_key": "sk-ant-...",
      "auth": "x-api-key"
    },
    "openrouter": {
      "base_url": "https://openrouter.ai/api/v1",
      "api_key": "sk-or-...",
      "auth": "bearer"
    },
    "ollama": {
      "base_url": "http://ollama:11434/v1",
      "auth": "none"
    }
  }
}
```

Auth modes: `bearer` (Authorization header), `x-api-key` (Anthropic's header), `none` (local models like Ollama).

## Operator dashboard

The built-in web UI on `:8081` has three pages:

- **Providers** (`/`) — Add, update, and delete upstream provider configs. Shows a routing diagram explaining how model names map to providers.
- **Pod** (`/pod`) — Lists registered agents with their type, request count, cost, and models used.
- **Costs** (`/costs`) — Real-time spend dashboard. Total spend banner, per-agent breakdown with nested model detail rows.
- **Costs API** (`GET /costs/api`) — JSON endpoint for Grafana, alerting, or scripts.

Cost data lives in memory and resets on restart. The structured JSON logs on stdout are the durable audit record.

## API endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/chat/completions` | OpenAI-compatible chat proxy (`:8080`) |
| `POST` | `/v1/messages` | Anthropic native messages proxy (`:8080`) |
| `GET` | `/health` | Health check — returns `{"ok": true}` (`:8080`) |

## Structured logging

Every request produces a JSON log line on stdout:

```json
{"ts":"2026-02-27T15:23:45Z","claw_id":"tiverton","type":"response","model":"anthropic/claude-sonnet-4","latency_ms":1250,"status_code":200,"tokens_in":100,"tokens_out":50,"cost_usd":0.0105,"intervention":null}
```

The `intervention` field is always `null` in passthrough mode. Future policy proxies (`cllama-policy`) will populate it when they drop, amend, or reroute requests.

These logs are designed for `docker compose logs`, fleet telemetry pipelines, and the `claw audit` command.

## Part of Clawdapus

This proxy is one layer in the [Clawdapus](https://github.com/mostlydev/clawdapus) stack — infrastructure-layer governance for AI agent containers. The full picture:

```
Clawfile          → extended Dockerfile (agent image)
claw-pod.yml      → extended docker-compose (agent fleet)
claw compose up   → builds images, generates configs, wires cllama, runs fleet
cllama-passthrough → LLM proxy with credential starvation + cost tracking
```

In a Clawdapus pod, `claw compose up` handles everything automatically:
- Generates per-agent bearer tokens via `crypto/rand`
- Rewrites agent model configs to point at the proxy
- Mounts agent contracts and context into the proxy
- Injects real provider keys only into the proxy's env (never the agent's)

You can also run `cllama-passthrough` standalone — just set up the context directory and `providers.json` manually, point your agents at it, and go.

## What's next

- **Budget enforcement** — Hard spend caps per agent. When exceeded, the proxy returns `429` instead of forwarding. The agent's budget is a configuration concern, not a prompt concern.
- **Model allowlisting** — Restrict which models each agent can request.
- **Persistent cost state** — Survive restarts without losing the running total.
- **`cllama-policy`** — A second proxy type that reads the agent's behavioral contract and makes allow/deny/amend decisions on the LLM traffic. The passthrough establishes the plumbing; the policy proxy adds the brain.
