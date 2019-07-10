import sys
import argparse
import re
import datetime

from requests_html import HTMLSession

steamGamePage = "https://store.steampowered.com/app/{}/"

def parseGamePage(session, app) :
    link = steamGamePage.format(app)
    print (link)
    # set cookies
    cookies = {"wants_mature_content":'1', "birthtime":"60368401", "lastagecheckage":"1-0-1972"}
    gameAppPage = session.get(link, cookies=cookies)
    if gameAppPage.url != link :
        # game removed?
        return True

    releaseDateElem = gameAppPage.html.find('div[class="date"]', first=True)
    if releaseDateElem == None :
        # game not released yet - skip
        return False

    try:
        releaseDate = datetime.datetime.strptime(releaseDateElem.text, "%d %b, %Y")
    except ValueError:
        # no release date - skip
        return False

    print (str(releaseDate))
    now = datetime.datetime.now()
    if (now - releaseDate < datetime.timedelta(days=365)):
        # very fresh game to decide
        return False

    ratingValue = gameAppPage.html.find('meta[itemprop="ratingValue"]', first=True)
    if ratingValue == None :
        # game not yet released or do not have rating
        return False

    ratingValueInt = int(ratingValue.attrs["content"])
    if (ratingValueInt == 10) :
        # return false - this is cool game
        return False

    if (ratingValueInt < 7 and ratingValueInt != 0) :
        # very suspicios game - need to check
        return True

    return False
    #app = re.findall("\d{1,7}", link)[0]

def main(argv) :
  print ("Start")

  # some defines
  # steamDbPage = "https://steamdb.info/app/{}/"
  steamFollowedGamesPage = "https://steamcommunity.com/id/{}/followedgames/"

  parser = argparse.ArgumentParser()
  parser.add_argument("profile", help="steam profile for parse followed games")
  args = parser.parse_args()
  print ("Profile '{}'".format(args.profile))

  print ("Parse Steam Followed Games List")
  session = HTMLSession()
  r = session.get(steamFollowedGamesPage.format(args.profile))
  
  pattern = "\d{1,8}"
  aaa = {}
  for ddd in r.html.find("div[class='gameListRowItemName']") :
      lll = ddd.find("a", first=True)
      app = re.search(pattern, lll.attrs["href"])[0]
      aaa[app] = lll.text

  print ("Parse {} followed games".format(len(aaa)))

  # single thread solution
  gamesToCheck = {steamGamePage.format(app):name for app,name in aaa.items() if parseGamePage(session, app)}

  # print result
  for link,name in gamesToCheck.items() :
    print ("{}\t\t{}".format(link, name))

if __name__ == '__main__' :
    main(sys.argv[1:])
