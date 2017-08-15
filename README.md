# sgbot

SteamGifts bot to enter for whitelisted games

I wish I would win all my followed and wishlisted games ;) I don't want win all the games in SG and this bot will apply only for whilelisted games (in future only for followed and wishlist in steam profile)

This is my first project in GO (any suggestions send in issues)

This is python implementation which inspired me to do this
https://github.com/theWaR13/SteamGiveawayManager

Next things to do
* Add config.json with steam profile link and parse followed and wishlisted games from it (remove gameslist.json? or leave it for additional games)
* Use gzip parser to work with gzip http answers
* Add some timeouts (as python impl - I think more "human"-behaviour)
* Test for proper daemon work
* Add some AI(?) - priority maps. Enter for wishlisted rather than followed

External imports
* https://github.com/PuerkitoBio/goquery - useful jquery-like selectors for HTML documents
* https://github.com/takama/daemon - golang daemon
