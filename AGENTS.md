# AGENTS.md

Guidance for AI coding agents working in this repository.

## What this project is

`sgbot` — a bot that automatically enters **SteamGifts.com** giveaways, but **only for whitelisted games** (Steam wishlist + followed games + an optional manual list from the database). A second bot, `gogbot`, claims giveaways on **GOG.com**.

The primary (and current) deployment target is **Yandex.Cloud serverless cloud functions** backed by a **YDB** (YandexDB) serverless database. There is no local daemon, no web server, and no `main()` in the production code paths — the cloud-function runtime invokes named entry points.

sgbot scrapes through the **ZenRows** API and holds a *pool* of ZenRows keys in the `keys` YDB table; when a key's allowance is spent (402/AUTH004) the bot rotates to the next key.

## Hard constraints on this host

- **There is no Go toolchain on this host.** Do NOT run `go build`, `go test`, `go vet`, `go run`, `gofmt`, or any other Go command. Work with static analysis: read code, reason about compilation (imports, types, duplicate declarations), and produce code changes. Verify changes visually, line by line.
- **Never commit real credentials.** The committed versions of `sgbot/bot-func_test.go`, `gogbot/test.go` and `.vscode/launch.json` (empty `args`) use placeholders/empties on purpose. Real keys/cookies may exist in the *working tree* for local debugging (test files, `launch.json` args) — they must stay uncommitted (check `git diff` before committing).
- **Tests here are live, side-effecting E2E tests, not unit tests.** `TestBotFunc` (sgbot) hits the real Steam API and real SteamGifts pages through the paid ZenRows proxy and *actually enters giveaways with a real account*. Even on a host with Go installed, never run the test suite "just to check" — it consumes ZenRows quota and acts on a live account. `.vscode/launch.json` has a "Launch bot manual trigger" configuration that does exactly this (dlv on `TestBotFunc`).

## Repository layout

Three **independent Go modules** (each with its own `go.mod`/`go.sum`), plus auxiliary material:

| Path | Module | Go | Purpose |
|---|---|---|---|
| `sgbot/` | `sgbot.mem4.me/sgbot` | 1.23 | Main bot: 3 cloud-function entry points in one `package main` |
| `gogbot/` | `gogbot.mem4.me/gogbot` | 1.17 | GOG giveaway claimer: cloud function + local debug runner |
| `checkdp/` | `chkchdp` | 1.23.12 | **Untracked, scratch-only** standalone `main.go` to debug the ZenRows SDK against steamgifts.com. Not part of deployment. Contains hardcoded keys — do not commit. |
| `README.md` | — | — | Human-facing deploy guide, kept in sync with the current flow (incl. the `keys` table) |
| `tools/` | — | — | Python helper scripts (`fg.py` + `requirements.txt`), not part of any Go module or deployment |

### `tools/` — Python helpers

- `tools/fg.py` — manual whitelist-triage script (needs `requests`, see `tools/requirements.txt`; run ad-hoc: `python3 tools/fg.py <steam-id> <api-key>`). Fetches the profile's **followed games** list via the Steam Web API (`IStoreService/GetGamesFollowed`, key passed as `id` param, numeric steam id as `steamid` param) and, for each game, talks to the **Steam Storefront API only**: `api/appdetails` (name, release date, genres) and `appreviews` (positive/total review ratio). It skips unreleased games, games released less than year ago, and Early Access (genre id 70). It prints two lists: games to check for *removing* from followed (review ratio < 83% or fewer than 10 reviews), and games to *promote* from followed to wishlist (ratio > 93%). Purpose: shrink the whitelist the bot enters giveaways for. Nothing in the Go code calls it.

### `sgbot/` file map (single `package main`, no `main()`)

- `thebot.go` — core logic. `TheBot` struct; Steam API calls (`fetchWishlist`, `fetchFollowedList` via `api.steampowered.com`); SteamGifts page fetch + parse through the **ZenRows** client (`scraperapi`; `JSRender: true`, `WaitForSelector: "body"` — this is how the bot bypasses Cloudflare); giveaway parsing with goquery; token extraction; `/ajax.php` entry POST. On a ZenRows **402/AUTH004** answer (`isAuth004`) the bot rotates the key rotor, recreates the `scraperapi` client and **retries the same request**; when the rotor is exhausted it returns `ErrKeysExhausted` and the run stops.
- `rotor.go` — `Rotor`: a rotating list of strings (the ZenRows key pool). Current value is always position 0; possible rotations equal the initial list length (so each key gets one pass per run before exhaustion); empty list → `ErrEmptyRotor`; `Order()` returns a copy of the current order (used to persist the rotated order back to YDB).
- `bot-func.go` — **cloud function entry `RunSGBOTFunc(ctx) (*Response, error)`**. Reads env, loads cookies (`domain LIKE "%steam%"`), games, and all `zenrows` keys from YDB, runs the bot, writes errors + digest to YDB. `RunBot(bot *TheBot, req *Request)` — the *caller* creates and keeps the `TheBot` instance so that after the run (any finish) the function compares the rotor's current key order with the one read from the DB and, if a rotation happened, persists the new order in one transaction (delete old `zenrows` rows, insert the rotated ones with ids re-numbered from 1).
- `bot-init-func.go` — **entry `RunInitBotDB`**: creates four YDB tables — `games(id:uint64 PK, name:string)`, `cookies(name, value, domain, path, PK(name, domain))`, `digest(message:UTF8 PK)`, `keys(id:uint64 PK, type:string, value:string)`.
- `digest-func.go` — **entry `SendDigest`**: drains the `digest` table and emails it via gomail.
- `sorter.go` — generic `By`/`timeSorter` sort helpers for `GiveAway` by time.
- `func-response.go` — `Response{StatusCode int}` (the function return type).
- `bot-func_test.go` — live E2E test (see constraint above). Keep credentials as placeholders.

### `gogbot/` file map (single `package main`)

- `thebot.go` — plain `http.Client` (no ZenRows) with a cookiejar; claims via `GET https://www.gog.com/giveaway/claim`. Status-code driven: 200 = nothing to claim, 201 = claimed (→ digest), 401 = cookie expired/invalid (→ digest).
- `bot-func.go` — **entry `RunGOGBOTFunc`**: loads cookies with `domain LIKE "%gog%"` from YDB, runs the claim, writes digest.
- `response.go` — `Response` (duplicated from sgbot; the modules are independent on purpose).
- `test.go` — local manual runner (`main()`), reads cookies from `assets/cookies..json` (yes, the filename in the const has a double dot — keep in sync if changed). The file is a JSON object mapping cookie name → `"<value>:<domain>:<path>"`, e.g. `"gog-al": "<VALUE>:gog.com:/"`. `assets/` is a local untracked drop point for it.

## How deployment works — read this before adding files

Each cloud function is deployed as a **zip produced by a shell script** in the repo root:

- `yandex.botinit-func.deploy.sh` → `sgbot/`: `bot-init-func.go go.mod func-response.go`
- `yandex.sgbot-func.deploy.sh` → `sgbot/`: `bot-func.go thebot.go go.mod func-response.go sorter.go rotor.go`
- `yandex.digest-func.deploy.sh` → `sgbot/`: `digest-func.go go.mod func-response.go`
- `yandex.gogbot-func.deploy.sh` → `gogbot/`: `bot-func.go thebot.go go.mod response.go`

**The zip file list is hardcoded in each script.** If you add a new `.go` file to a module, you must add it to **every** deploy script whose function needs to see it — otherwise the cloud function will fail to compile (missing declaration / undefined symbol). `func-response.go`/`response.go` exist precisely so each minimal zip compiles standalone.

Cloud-function specifics (Yandex serverless):
- Entry signature: `func(ctx context.Context) (*Response, error)`.
- YDB connection is hardcoded to `grpcs://ydb.serverless.yandexcloud.net:2135/?database=$YDB_DATABASE`; IAM via `yc.InstanceServiceAccount()`.
- Env vars: `YDB_DATABASE` (all functions), `STEAM_PROFILE`, `STEAM_API_KEY` (bot) — the ZenRows key is **not** an env var; it lives in the `keys` table (see below). `MAILER_SMTP`, `MAILER_PORT`, `MAILER_AUTH_NAME`, `MAILER_AUTH_PWD`, `MAILER_SUBJECT`, `MAILER_RECIPIENT` (digest).

## Key behaviors & gotchas (don't "fix" without understanding)

- **ZenRows key rotation**: all `keys` rows with `type = 'zenrows'` (ordered by `id` — the `id` order *is* the usage order) go into the `Rotor`; the first key is used as-is. On 402/AUTH004 the bot rotates, recreates the client and retries the *same* request. When no rotations are left the run fails with `ErrKeysExhausted` (→ digest). After the run, if the order changed, the bot writes it back in one serializable transaction: delete all `zenrows` rows, re-insert with ids re-numbered from 1. This is how a spent key gets pushed to the end for the next scheduled run.
- **HTTP 422 from `/ajax.php` is treated as success** in `postRequest` (means "already entered"). The log line says "internal error" but the code returns success — intentional.
- The bot processes the **wishlist page first** (window: giveaways ending within ~5 weeks) and the **main page second** (window: 1 hour). It sleeps 1–3s between entries to mimic human behavior.
- Whitelist = Steam wishlist ∪ followed games ∪ `games` table. An empty union is a hard error ("empty white list") and is reported to the digest.
- **Digest race**: the digest function *deletes* what it reads. The README explicitly warns not to run the digest function and a bot check at the same time.
- **SteamGifts HTML selectors are the brittle surface**: `div.giveaway__row-outer-wrap`, `a.giveaway__heading__name`, `a.giveaway__icon[target='_blank']`, `span[data-timestamp]`, token from `div.js__logout` `data-form` (last 32 chars). SG site redesigns break the bot silently (parse returns 0 giveaways). If touching parsers, keep `errlog` fallbacks.
- Cookies are matched to requests by **exact domain equality** (`k.Domain != pageURL.Host` → skipped).
- gogbot stores one special cookie, `gog-al` (domain `gog.com`), in the same `cookies` table sgbot uses.

## Legacy artifacts still in tree

- `.devcontainer/` — plain Go 1.23 dev container (gopls + dlv); a sensible way to get a Go toolchain on a host that doesn't have one.
- `.vscode/launch.json` — launch configs for `fg.py` (debugpy), `checkdp`, and a "bot manual trigger" (dlv on `TestBotFunc`). The committed `args` are empty; real values may appear in the working tree — never commit them.
- `assets/` — no longer holds tracked examples; it is the local, untracked drop point for `gogbot/test.go`'s `cookies..json` (secrets — never commit it).

## Conventions

- Go style follows what's in the tree: `package main`, tab indentation, `stdlog`/`errlog` loggers (stdout/stderr) in sgbot, `fmt.Println` for function-level logs in the cloud-function code. Match the surrounding file when adding code.
- Error handling in function entry points: return `(nil, err)` for fatal YDB/env problems; bot-level errors get written to the `digest` table so the mailer surfaces them.
- When modifying a module, keep its `go.mod`/`go.sum` self-consistent (new direct deps go in the `require` block; do not upgrade transitive deps opportunistically — the Yandex functions pin old SDK versions for a reason: the serverless Go runtime).
- Git: the repo is currently in a **detached HEAD** state (at `58c901c`, the "api-key-from-db" merge). Make changes on a named branch, not in detached HEAD. `checkdp/` is untracked on purpose (scratch with secrets).

## Suggested verification workflow (no Go available)

1. For each module you touch, grep the whole module for the symbols you changed (it's small — one package each).
2. Mentally compile: same-package files share identifiers, so a helper added in one file is visible in all of them *within the same deploy zip* — cross-check against the zip file list.
3. Check `git diff` for accidental secret material before any commit.
