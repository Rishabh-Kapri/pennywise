# lumo-tamer on Railway

[lumo-tamer](https://github.com/ZeroTricks/lumo-tamer) fronts [Proton Lumo](https://lumo.proton.me/)
with an OpenAI-compatible API. Deployed here, it gives cipher a `lumo` provider
that costs Proton-subscription money instead of OpenRouter credits.

This directory holds only what Railway needs: a Dockerfile that builds a pinned
upstream revision, and an entrypoint that renders lumo-tamer's `config.yaml`
from environment variables. No upstream source is vendored.

## What this works well for

`EMAIL_PIPELINE_PROVIDERS` — the extraction and classification steps send short
prompts, expect JSON back, and use no tools. That is what Lumo is good at.

Two things it does **not** replace:

- **Embeddings.** Lumo serves no embedding endpoint. `EMAIL_EMBEDDING_PROVIDERS`
  must stay on ollama or OpenRouter (`bge-m3`, 1024 dims) — a different model
  invalidates every stored pgvector row.
- **The chat agent, realistically.** Lumo has no native tool calling; lumo-tamer
  emulates it by describing tools in the prompt and parsing JSON back out
  (`LUMO_CUSTOM_TOOLS=true`, off by default). Prompt caching does nothing here
  either, and lumo-tamer's own config warns Lumo starts struggling around ~22.5K
  tokens. Point `AGENT_PROVIDER` at it if you want, but expect worse tool-calling
  than Anthropic or OpenAI.

## Before you start

- A Proton account. The free tier caps daily chats, and the pipeline spends 2–3
  calls per email — Lumo Plus is the realistic option.
- Node.js 18+ and Go 1.24+ locally, for the one-time authentication step.
- lumo-tamer is an unofficial client, and using it may violate Proton's terms of
  service. Proton has also announced an official Lumo API, at which point most of
  this goes away.

## 1. Authenticate locally

Authentication is interactive — Proton password, optionally 2FA, sometimes a
CAPTCHA — and it shells out to a Go SRP binary. None of that can happen during a
Railway deploy, so you do it once on your machine and ship the encrypted result.

```bash
git clone https://github.com/ZeroTricks/lumo-tamer.git
cd lumo-tamer
npm install && npm run build:all

# The vault key MUST be a file, not your OS keychain — the container has no
# keychain, and a keychain-held key cannot be exported later.
mkdir -p secrets
openssl rand -base64 32 > secrets/lumo-vault-key
chmod 600 secrets/lumo-vault-key

cat > config.yaml <<'YAML'
auth:
  method: "login"
  vault:
    path: "sessions/vault.enc"
    keyFilePath: "./secrets/lumo-vault-key"
server:
  apiKey: "local-only"
YAML

npx tamer auth login
```

Getting a CAPTCHA? Log in to Proton in a normal browser from the same IP first,
then retry.

This writes `sessions/vault.enc`, encrypted with `secrets/lumo-vault-key`. Both
halves are needed — a vault without its key is unreadable.

Now print the two values Railway needs:

```bash
cat secrets/lumo-vault-key      # -> LUMO_VAULT_KEY
base64 -w0 sessions/vault.enc   # -> LUMO_VAULT_SEED   (macOS: base64 -i sessions/vault.enc)
```

Also generate the key that guards the deployment itself:

```bash
openssl rand -hex 32            # -> LUMO_API_KEY
```

## 2. Create the Railway service

From the Railway dashboard, in the project cipher runs in (or a new one):

1. **New → GitHub Repo →** this repository.
2. **Settings → Build → Root Directory:** `deploy/lumo-tamer`. Railway picks up
   `railway.json` and the Dockerfile from there.
3. **Settings → Volumes → Add volume**, mount path `/app/sessions`.
   This is not optional. Tokens auto-refresh roughly every 20 hours and Proton
   **rotates the refresh token** each time, rewriting the vault. Without a
   volume, every redeploy restores the seeded vault whose refresh token is
   already dead, and you re-authenticate by hand.
4. **Settings → Networking:** see [Exposure](#exposure) before generating a
   public domain.

Keep it at one replica. Two instances would share one Proton session and race
each other's token refresh — `railway.json` pins `numReplicas: 1`.

## 3. Set variables

| Variable | Required | Notes |
|---|---|---|
| `LUMO_API_KEY` | yes | Bearer key clients must send. The only thing guarding this service. |
| `LUMO_VAULT_KEY` | yes | Contents of `secrets/lumo-vault-key` (base64, 44 chars). |
| `LUMO_VAULT_SEED` | first boot | base64 of `sessions/vault.enc`. Seeds the volume once; ignored once a vault exists. |
| `PORT` | injected | Railway sets it; the entrypoint binds whatever it says. |
| `LUMO_DEFAULT_MODEL_TIER` | no | `auto` (default), `lumo-lite`, `lumo-max`. Only applies when a request omits `model`. |
| `LUMO_REASONING_DEFAULT` | no | `none` (default) or `high`. |
| `LUMO_BODY_LIMIT` | no | Default `360kb`. Raise for larger payloads, but Lumo's context is the real ceiling. |
| `LUMO_CUSTOM_TOOLS` | no | `true` enables emulated tool calling. Needed only for the chat agent. |
| `LUMO_ENABLE_WEB_SEARCH` | no | Lumo's native web search. Default `false`. |
| `LUMO_LOG_LEVEL` | no | `trace`…`fatal`, default `info`. |
| `LUMO_LOG_MESSAGE_CONTENT` | no | `true` logs prompt/response bodies. Default `false` — leave it. |
| `LUMO_METRICS` | no | `true` exposes unauthenticated `/metrics`. |

`config.yaml` is regenerated from these on every boot, so changing a variable and
redeploying is all it takes. The volume holds tokens, never configuration.

## 4. Verify

```bash
curl https://<service>.up.railway.app/health
# {"status":"ok","queue":{...}}

curl https://<service>.up.railway.app/v1/models -H "Authorization: Bearer $LUMO_API_KEY"
# {"object":"list","data":[{"id":"lumo"},{"id":"lumo-lite"},{"id":"lumo-max"}]}

curl https://<service>.up.railway.app/v1/responses \
  -H "Authorization: Bearer $LUMO_API_KEY" -H 'content-type: application/json' \
  -d '{"model":"lumo","input":[{"role":"user","content":"say hi"}]}'
```

`/health` answers without a key and is what Railway's healthcheck uses, so it
coming up green proves the process started — not that authentication worked.
The third call is the one that proves the Proton session is live.

## 5. Point cipher at it

```bash
LUMO_BASE_URL=https://<service>.up.railway.app   # or http://<service>.railway.internal:${PORT}
LUMO_API_KEY=<same key as above>
EMAIL_PIPELINE_PROVIDERS=ollama=gemma4:12b,lumo=lumo-max
```

Setting `LUMO_BASE_URL` is what registers the provider — leave it empty and
nothing changes. The URL may include or omit the trailing `/v1`; cipher
normalizes it. Keep a second target in the chain: any provider that fails falls
through to the next, so a dead Proton session degrades to whatever follows it
instead of stalling ingestion.

To send agent chat there too: `AGENT_PROVIDER=lumo`, and set
`LUMO_CUSTOM_TOOLS=true` on this service — with tool emulation off, the agent
gets prose where it expects tool calls and every run fails.

## Exposure

Upstream's README says plainly: don't put lumo-tamer on the internet. A Railway
public domain does exactly that, with `LUMO_API_KEY` as the only guard, and
`/health` and `/metrics` answering unauthenticated.

If cipher runs on Railway in the same project, skip the public domain entirely
and use private networking — `http://<service>.railway.internal:${PORT}`. Nothing
outside the project can reach it, and traffic stays off the public internet.

If cipher runs elsewhere (the Unraid compose stack), you need the public domain,
so treat `LUMO_API_KEY` as a real credential: 32+ random bytes, rotated if it
ever lands anywhere it shouldn't.

## Re-authenticating

The vault dies if the refresh token is ever rotated away without the new one
being saved — a volume that got wiped, a rolled-back deploy, a revoked session.
Symptoms: 401s from Proton in the logs, `/health` still green.

Two ways back:

1. **Re-run step 1 locally**, set the new `LUMO_VAULT_SEED`, and delete the
   volume's `vault.enc` so the seed applies (`railway ssh`, then
   `rm /app/sessions/vault.enc`, then redeploy).
2. **Authenticate in place:** `railway ssh` into the service and run
   `tamer auth login`. The Go binary ships in the image. Expect CAPTCHA
   friction — you are logging in from a datacenter IP that has never seen your
   account, which is exactly the pattern Proton's abuse detection looks for.

Rotating `LUMO_VAULT_KEY` means re-encrypting the vault: authenticate locally
against the new key file and reseed. The key and the vault are a matched pair; a
mismatch reads as "wrong key or corrupted file".

## Upgrading upstream

`LUMO_TAMER_REF` in the Dockerfile pins a commit. To move:

```bash
# in a lumo-tamer clone
git fetch && git log --oneline -5 origin/main
git diff <current-ref>..<new-ref> -- config.defaults.yaml
```

Check that diff — `entrypoint.sh` writes config keys that upstream validates
with a strict schema, and a renamed or removed key turns into a startup crash
loop. Then bump the ARG and redeploy.
