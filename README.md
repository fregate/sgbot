# sgbot

SteamGifts bot to enter for whitelisted games

I wish I would win all my followed and wishlisted games ;) I don't want win all the games in SG and this bot will apply only for whilelisted games.

This is my first project in GO (any suggestions send in issues)

This is python implementation which inspired me to do this
https://github.com/theWaR13/SteamGiveawayManager

Next things to do
* Work with /sub/ giveaways. Parse all steam games (do not use steam auth or api - if any), parse steam sub page, decide to enter or not
** Sometimes it redirects from https://store.steampowered.com/sub/127633/ (ie) to https://store.steampowered.com/agecheck/sub/127633# , so it has to set cookie 
"wants_mature_content"=1 for store.steampowered.com domain, path = '/' (in client options)
and
"birthtime"=-729000000	domain:store.steampowered.com	path:/

# Preparations
1. **config.json** - bot config. Optional
 + profile - If you want to parse your steam profile. You need only name, not whole URL. Optional.
 + mail - set smtp settings for sends some notifications (optional)
   * smtp - smtp server
   * port - smtp server port
   * username - smtp authorization username (mail will be sent from this email)
   * password - smtp auth password (*not tested without auth*)
 + digest - send daily digest for the previous day (or in panic message - current digest)
 + subjecttag - tag in mail subject (i.e. [SG_BOT])
 + recipient - mail will be send to this email
 + *if there is no "recipient" or "mail" not fiiled properly - no mail at all*
2. **gameslist.json** - Optional. fill it with games. "SteamID":"Name", *Name - optional: used only for logs (you can leave it empty "")* If no games loaded (profile + list) - bot stop
3. **cookies.json** Required. You need to autorize in SG through browser, go to the DevTools in it and copy all cookies (I think only session cookie works, but anyway). "Name":"Value"
4. To run as daemon (tested only for linux)
  * sudo ./bot install
  * Set service working dir for ./bot path (/etc/systemd/system/sgbotservice.service -> [Service] -> WorkingDirectory=/path/to/executable
  * (sudo) systemctl daemon-reload
  * sudo service sgbotservice start
  * sudo service sgbotservice status
  * profit!
  
# External imports
* https://github.com/PuerkitoBio/goquery - useful jquery-like selectors for HTML documents
* https://github.com/takama/daemon - golang daemon
* http://gopkg.in/gomail.v2 - mailer for spam
