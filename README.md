# sgbot

SteamGifts bot to enter only for whitelisted games

I wish I would win all my followed and wishlisted games ;) I don't want win all the games in SG and this bot will apply only for whilelisted games.

This is my first project in GO (and there no some goish features like channels and coroutines - just syntax)

This is python implementation which inspired me to do this
https://github.com/theWaR13/SteamGiveawayManager

# Preparations
1. **config.json** - bot config. Optional
 + `profile` - If you want to parse your steam profile. You need only name, not whole URL. Optional.
 + `mail` - set smtp settings for sends some notifications (optional)
   * `smtp` - smtp server
   * `port-num` - smtp server port
   * `username` - smtp authorization username (mail will be sent from this email)
   * `password` - smtp auth password (*not tested without auth*)
   * `subjecttag` - tag in mail subject (i.e. [SG_BOT])
   * `recipient` - mail will be send to this email
 + `digest` - send daily digest for the previous day (or in panic message - current digest)
 + *if there is no "recipient" or "mail" not filled properly - no mail at all*
 simple server config (optional). these paramters can'be changed through web ui. to apply changes bot must be restarted
 + `httpauth` - http simple auth login
 + `httppwd` - http simple auth password
 + `web-port-num` - http server listening port (default 8080) (don't forget to open it in firewall!)
2. **gameslist.json** - Optional. Fill it with games. "SteamID":"Name", *Name - optional, all game titles takes from gifts page: you can leave it empty: "")* If no games loaded (profile + list) - bot stops
3. **cookies.json** Required. You need to autorize in SG through browser, go to the DevTools in it and copy all cookies (I think only session cookie works, but anyway). "Name":"Value:Domain:Path" (separated by colon)
If you wish parse giveaways that points to /sub/ steam pages with age check, you have to set these cookies:
    "wants_mature_content": "1:store.steampowered.com:/",
    "birthtime": "60368401:store.steampowered.com:/", # about 1972/4/4
    "lastagecheckage": "1-0-1972:store.steampowered.com:/"

4. To run as daemon (tested only for linux)
  * sudo ./bot install
  * Set service working dir for ./bot path (/etc/systemd/system/sgbotservice.service -> [Service] -> WorkingDirectory=/path/to/executable
  * (sudo) systemctl daemon-reload
  * sudo service sgbotservice start
  * service sgbotservice status
  * profit!

# SGBot as a cloud function
If you have some cloud functions service (AWS Lambda, Yandex.Cloud) you could try to install this bot as cloud function. At this point you can install it on Yandex.Cloud (as did I).
Frankly, there is 2 cloud functions: bot who checks and email sender. And I not implemented (yet?) spectacular and rich web-configuration script (because you can change cookies, games and other parameters directly inside cloud console).

## Create DB for games, cookies and digest
1. Create YandexDB (YDB) serverless database
2. Create 3 tables: `games (id:uint64, name:string)`, `cookies (name:string, domain:string, path:string, value:string)`, `digest (message:string)`
_(TODO: make one-timer function to create tables)_

## Create bot function
1. Run `yandex.bot-func.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17, set 128M, 60sec timeout, set `bot-func.RunSGBOTFunc` as entry point
3. Create service account with editor privelegies for YDB
4. Set `STEAM_PROFILE` and `YDB_DATABASE` (this is location from YDB) environment variables
5. Finish function creation
6. Create trigger for schedule function invokation (hourly - but you can check as you wish)
7. Create service account (or add to existing serverless.invoker role)
8. It's have to work!

## Create digest function
1. Run `yandex.digest-bot.deploy.sh` - it prepares all mandatory files
2. Create function from zip archive, choose Go/1.17, set 128M, 5sec timeout, set `digest-func.SendDigest` as entry point
3. Create service account with editor privelegies for YDB (or use existing)
4. Set `MAILER_SMTP`, `MAILER_PORT`, `MAILER_AUTH_NAME`, `MAILER_AUTH_PWD`, `MAILER_SUBJECT`, `MAILER_RECIPIENT` environment variables for mailer creation and `YDB_DATABASE` for DB connection
5. Finish function creation
6. Create trigger for schedule function invokation (daily - but you can send as you wish)
7. Create (select) service account with serverless.invoker role
8. It's have to work!

# External imports
* https://github.com/PuerkitoBio/goquery - useful jquery-like selectors for HTML documents
* https://github.com/takama/daemon - golang daemon
* http://gopkg.in/gomail.v2 - mailer for spam
* https://github.com/yandex-cloud/go-sdk - using as cloud function
* https://github.com/ydb-platform/ydb-go-sdk - store data for cloud function

