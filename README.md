# sgbot

SteamGifts bot to enter for whitelisted games

I wish I would win all my followed and wishlisted games ;) I don't want win all the games in SG and this bot will apply only for whilelisted games (in future only for followed and wishlist in steam profile)

This is my first project in GO (any suggestions send in issues)

This is python implementation which inspired me to do this
https://github.com/theWaR13/SteamGiveawayManager

Next things to do
* ~~Add config.json with steam profile link and parse followed and wishlisted games from it (remove gameslist.json? or leave it for additional games)~~
* Use gzip parser to work with gzip http answers
* ~~Add some timeouts (as python impl - I think more "human"-behavior)~~
* ~~Test for proper daemon work in linux~~ 
* Add some AI(?) - priority maps. Enter for wishlisted rather than followed
* Add some notifications to user through email (won gift, need to refresh cookies, sync account, etc)
* Reload lists (or parse account) on the fly (without daemon restart)

# Preparations
1. If you want to parse your steam profile - set it in config.json "profile". You need only name, not whole URL. Or leave it empty (remove).
2. Fill gameslist.json with games. "SteamID":"Name", Name - optional: used only for logs (leave it empty "" or remove)
3. Fill cookies.json. You need to autorize in SG through browser, go to the DevTools in it and copy all cookies (I think only session cookie works, but anyway). "Name":"Value"
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
