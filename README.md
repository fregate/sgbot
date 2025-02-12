# sgbot

SteamGifts bot to enter only for whitelisted games

I wish I would win all my followed and wishlisted games ;) I don't want win all the games in SG and this bot will apply only for whilelisted games.

This is my first project in GO (and there no some goish features like channels and coroutines - just syntax)

This is python implementation which inspired me to do this
https://github.com/theWaR13/SteamGiveawayManager

## SGBot as a cloud function
If you have some cloud functions service (AWS Lambda, Yandex.Cloud, _etc_) you could try to install this bot as cloud function. At this point you can install it on Yandex.Cloud (as I did).
Frankly, there is 3 cloud functions: bot which checks, email sender and script with db seeding.

### Create DB for games, cookies and digest
1. Create YandexDB (YDB) serverless database
2. Copy DB 'location'

### Create bot init function
1. Run `yandex.botinit-func.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17, set 128M, 60sec timeout, set `bot-init-func.RunInitBotDB` as entry point
3. Create service account with editor privelegies for YDB
4. Set `YDB_DATABASE` (this is location from YDB) environment variables
5. Finish function creation
6. Run function once (test). It has to create 3 tables into YDB: `games (id:uint64, name:string)`, `cookies (name:string, domain:string, path:string, value:string)` and `digest (message:UTF8)`

### Create bot function
1. Run `yandex.sgbot-func.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17, set 128M, 60sec timeout, set `bot-func.RunSGBOTFunc` as entry point
3. Create service account with editor privelegies for YDB
4. Set `STEAM_PROFILE`, `STEAM_API_KEY` and `YDB_DATABASE` (this is location from YDB) environment variables
5. Finish function creation
6. Create trigger for schedule function invokation (hourly - but you can check as you wish)
7. Create service account (or add to existing serverless.invoker role)
8. It has to work!

### Create digest function
1. Run `yandex.digest-bot.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17, set 128M, 5sec timeout, set `digest-func.SendDigest` as entry point
3. Create service account with editor privelegies for YDB (or use existing)
4. Set `MAILER_SMTP`, `MAILER_PORT`, `MAILER_AUTH_NAME`, `MAILER_AUTH_PWD`, `MAILER_SUBJECT`, `MAILER_RECIPIENT` environment variables for mailer creation and `YDB_DATABASE` for DB connection
5. Finish function creation
6. Create trigger for schedule function invokation (daily - but you can send as you wish)
7. Create (select) service account with serverless.invoker role
8. It has to work!

# gogbot
Check GOG.com for giveaways (only for cloud functions)

## Prerquisites
Run bot-init (from sgbot *TODO: make this function and digest like shared function*) to create `cookie` database.
Create `digest` function too for receive emails.

## Deploy and run gogbot func
1. Run `yandex.gogbot-func.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17+, set 128M, 60sec timeout, set `bot-func.RunGOGBOTFunc` as an entry point
3. Create or use existed (created for `sgbot` for example) service account with editor privelegies for YDB
4. Set `YDB_DATABASE` (this is location from YDB) environment variables
5. Finish function creation
6. Create trigger for schedule function invokation (daily - but you can check as often as you wish)
7. Create service account (or add to existing serverless.invoker role)
8. Add necessary cookie `gog-al` to database with domain like gog.com
9. It has to work!

Bot writes something to log in 2 cases: first, if you won something, and second - if cookies are expires or invalid (401 - unauthorized). In other cases bot writes to log return code (to analyze if somethig will change).

# External imports
* https://github.com/PuerkitoBio/goquery - useful jquery-like selectors for HTML documents
* https://github.com/takama/daemon - golang daemon
* http://gopkg.in/gomail.v2 - mailer for spam
* https://github.com/yandex-cloud/go-sdk - using as cloud function
* https://github.com/ydb-platform/ydb-go-sdk - store data for cloud function
