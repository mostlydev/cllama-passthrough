# cllama-passthrough

**The reference governance proxy for Clawdapus.**

`cllama-passthrough` is a standalone, OpenAI-compatible proxy that acts as the foundational governance layer for AI agent containers deployed via [Clawdapus](https://github.com/mostlydev/clawdapus). 

In the Clawdapus architecture, agents run as untrusted workloads and are isolated via **credential starvation**. The agent never possesses real LLM provider API keys. Instead, all inference requests pass through the `cllama` proxy, which enforces identity, swaps in the real credentials, and routes the request to the upstream provider.

## Features

- **Transparent Proxying:** Exposes an OpenAI-compatible `/v1/chat/completions` endpoint on `:8080`.
- **Identity & Context Awareness:** Parses incoming `Bearer <agent-id>:<secret>` tokens and validates them against the agent's mounted Clawdapus context (`/claw/context/`).
- **Cost Accounting:** Tracks per-agent token usage and compute spend in real-time. Usage is automatically extracted from upstream responses, multiplied against an embedded pricing table, and tracked per agent, provider, and model.
- **Credential Starvation:** Holds the real API keys and injects them into outbound requests, ensuring the agent container never has direct access to provider credentials.
- **Multi-Provider Support:** Supports routing to OpenAI, Anthropic, OpenRouter, Ollama, and more.
- **Operator Web UI:** Provides a built-in management UI on `:8081` with a `/costs` dashboard for operators to review spend and manage provider API keys without touching environment variables.
- **Structured Audit Logging:** Emits structured JSON logs (RFC3339 timestamp, latency, agent ID, requested model, status codes, tokens, and `cost_usd`) for independent fleet telemetry and drift scoring.

## How it Works

1. **Inbound Request:** The Clawdapus runner makes an OpenAI-compatible request to `http://cllama-passthrough:8080/v1/chat/completions` using a dummy Bearer token (e.g., `Bearer bot-1:abc123hex`).
2. **Identity Verification:** `cllama-passthrough` reads the token, verifies the secret against the agent's mounted `metadata.json`, and confirms the agent's identity.
3. **Provider Routing:** It parses the requested model (e.g., `openai/gpt-4o` or `anthropic/claude-3-5-sonnet-20241022`), strips the provider prefix, and selects the correct upstream provider.
4. **Credential Swap:** The proxy swaps the dummy Bearer token for the real upstream API key configured in its registry.
5. **Streaming:** The request is forwarded, and the upstream response is transparently streamed back to the agent.

## Configuration

`cllama-passthrough` expects the following environment variables (automatically injected by Clawdapus when orchestrated):

- `CLAW_CONTEXT_ROOT`: Path to the directory containing agent contexts (default: `/claw/context`).
- `CLAW_AUTH_DIR`: Path to the directory where provider credentials are saved (default: `/claw/auth`).
- `LISTEN_ADDR`: API server bind address (default: `:8080`).
- `UI_ADDR`: Operator web UI bind address (default: `:8081`).

Provider keys can be initialized via environment variables (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY`), which will override any keys saved in the web UI.

## Building and Running

```bash
# Build the binary
go build -o cllama-passthrough ./cmd/cllama-passthrough

# Run locally
./cllama-passthrough
```

Or using Docker:

```bash
docker build -t ghcr.io/mostlydev/cllama-passthrough:latest .
```

## The cllama Standard

`cllama-passthrough` serves as the reference implementation for the `cllama` standard. It is a "passthrough" proxy because it performs no cognitive mutation of the request or response. Future proxies in a Clawdapus chain (e.g., `cllama-policy`) will intercept, evaluate, and potentially drop or rewrite LLM traffic based on the agent's behavioral contract.
