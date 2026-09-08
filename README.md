# sgbot

SteamGifts bot to enter only for whitelisted games

I wish I would win all my followed and wishlisted games ;) But I don't want to win all the games in SG and this bot will apply only for _whitelisted_ games.

## tools/fg.py - narrow your whitelist

A standalone Python helper (`python3 tools/fg.py <steam-profile>`, needs `requests_html`) that fetches your followed-games list and, per game, checks the Steam store rating and the SteamDB rating. It prints two lists: games worth *removing* from followed (low SteamDB rating or low Steam rating) and games worth *promoting* from followed to wishlist (top ratings). Use it to trim the whitelist the bot enters giveaways for. It is not called by any Go code.

## For agents working in this repo

See [AGENTS.md](AGENTS.md) - it documents the repo layout, the cloud-function deploy gotchas (each function is a zip with a hardcoded file list), and the host constraints (no Go toolchain, tests are live E2E and must not be run casually).

## SGBot as a cloud function
If you have a cloud-functions service (AWS Lambda, Yandex.Cloud, _etc_) you could try to install this bot as a cloud function. At this point you can install it on Yandex.Cloud (as I did).
Frankly, there are 3 cloud functions: a bot which checks, an email sender and a script with db seeding.

### Create DB for games, cookies, digest and keys
1. Create YandexDB (YDB) serverless database
2. Copy DB 'location'

### Create bot init function
1. Run `yandex.botinit-func.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17, set 128M, 60sec timeout, set `bot-init-func.RunInitBotDB` as entry point
3. Create service account with editor privileges for YDB
4. Set `YDB_DATABASE` (this is the location from YDB) environment variables
5. Finish function creation
6. Run function once (test). It has to create 4 tables into YDB: `games (id:uint64, name:string)`, `cookies (name:string, domain:string, path:string, value:string)`, `digest (message:UTF8)` and `keys (id:uint64, name:string, value:string)`

### Fill the `keys` table
The bot reads its Zenrows API key from the `keys` table (not from an environment variable). Insert one row per Zenrows key, all with `name = 'zenrows'`:
```sql
INSERT INTO keys (id, name, value) VALUES (1, 'zenrows', '<your Zenrows API key>');
```
Right now the bot takes only the first `zenrows` key it reads, so put the key you want to use first; adding more `zenrows` rows is the groundwork for key rotation later.

### Create bot function
1. Run `yandex.sgbot-func.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17, set 128M, 60sec timeout, set `bot-func.RunSGBOTFunc` as entry point
3. Create service account with editor privileges for YDB
4. Set `STEAM_PROFILE`, `STEAM_API_KEY` and `YDB_DATABASE` (this is the location from YDB) environment variables. The Zenrows key is no longer an environment variable - the bot reads it from the `keys` table (see "Fill the `keys` table" above)
5. Finish function creation
6. Create trigger for schedule function invocation (hourly - but you can check as you wish, but keep in mind about zenrows api restrictions)
7. Create service account (or add to existing serverless.invoker role)
8. It has to work!

### Create digest function
1. Run `yandex.digest-func.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17, set 128M, 5sec timeout, set `digest-func.SendDigest` as entry point
3. Create service account with editor privileges for YDB (or use existing)
4. Set `MAILER_SMTP`, `MAILER_PORT`, `MAILER_AUTH_NAME`, `MAILER_AUTH_PWD`, `MAILER_SUBJECT`, `MAILER_RECIPIENT` environment variables for mailer creation and `YDB_DATABASE` for DB connection
5. Finish function creation
6. Create trigger for schedule function invocation (daily - but you can send as you wish)
7. Create (select) service account with serverless.invoker role
8. It has to work!

# gogbot
Check GOG.com for giveaways (only for cloud functions)

## Prerequisites
Run bot-init (from sgbot *TODO: make this function and digest like shared function*) to create the `cookies` table.
Create the `digest` function too to receive emails.

## Deploy and run gogbot func
1. Run `yandex.gogbot-func.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17+, set 128M, 60sec timeout, set `bot-func.RunGOGBOTFunc` as an entry point
3. Create or use an existing (created for `sgbot` for example) service account with editor privileges for YDB
4. Set `YDB_DATABASE` (this is the location from YDB) environment variables
5. Finish function creation
6. Create trigger for schedule function invocation (daily - but you can check as often as you wish)
7. Create service account (or add to existing serverless.invoker role)
8. Add necessary cookie `gog-al` to database with domain like gog.com
9. It has to work!

Bot writes something to log in 2 cases: first, if you won something, and second - if cookies are expired or invalid (401 - unauthorized). In other cases bot writes to log return code (to analyze if something will change).

# External imports
* https://github.com/PuerkitoBio/goquery - useful jquery-like selectors for HTML documents
* http://gopkg.in/gomail.v2 - mailer for spam
* https://github.com/zenrows/zenrows-go-sdk - fetch SG pages through the ZenRows API (bypasses Cloudflare)
* https://github.com/yandex-cloud/go-sdk - using as cloud function
* https://github.com/ydb-platform/ydb-go-sdk - store data for cloud function
