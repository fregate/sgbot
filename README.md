# sgbot

SteamGifts bot to enter for whitelisted games

I wish I would win all my followed and wishlisted games ;) I don't want win all the games in SG and this bot will apply only for whilelisted games.

This is my first project in GO (any suggestions send in issues)

This is python implementation which inspired me to do this
https://github.com/theWaR13/SteamGiveawayManager

Next things to do
* ~~Add config.json with steam profile link and parse followed and wishlisted games from it (remove gameslist.json? or leave it for additional games)~~
* Use gzip parser to work with gzip http answers
* ~~Add some timeouts (as python impl - I think more "human"-behavior)~~ *Wait random amount of time before enter for giveaway (if bot can wait)*
* ~~Test for proper daemon work in linux~~ *Work as deamon*
* ~~Add some AI(?) - priority maps. Enter for wishlisted rather than followed~~ *If some points left - try to apply for wishlisted GAs*
* ~~Add some notifications to user through email (won gift, need to refresh cookies, sync account, etc)~~ *Reload cookies if cookies.json newer than loaded, sent errors but do not stop service*
* ~~Reload lists (or parse account) on the fly (without daemon restart)~~ *Reload list before every check*

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
