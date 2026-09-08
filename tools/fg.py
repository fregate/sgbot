import sys
import argparse
import re
import datetime

from requests_html import HTMLSession

steamGamePage = "https://store.steampowered.com/app/{}/"
totalItems = 0
currentItem = 0

wishlistGames = []

# Print iterations progress
def printProgressBar (iteration, total, prefix = 'Progress:', suffix = 'Complete', decimals = 1, length = 100, fill = '>'):
    """
    Call in a loop to create terminal progress bar
    @params:
        iteration   - Required  : current iteration (Int)
        total       - Required  : total iterations (Int)
        prefix      - Optional  : prefix string (Str)
        suffix      - Optional  : suffix string (Str)
        decimals    - Optional  : positive number of decimals in percent complete (Int)
        length      - Optional  : character length of bar (Int)
        fill        - Optional  : bar fill character (Str)
    """
    percent = ("{0:." + str(decimals) + "f}").format(100 * (iteration / float(total)))
    filledLength = int(length * iteration // total)
    bar = fill * filledLength + '-' * (length - filledLength)
    print('\r%s |%s| %s%% %s' % (prefix, bar, percent, suffix), end = '\r')
    # Print New Line on Complete
    if iteration == total: 
        print()

def parseGamePage(session, app) :
    global currentItem
    currentItem = currentItem + 1
    printProgressBar(currentItem, totalItems)
    link = steamGamePage.format(app)

    global wishlistGames

    # set cookies
    cookies = {"wants_mature_content":'1', "birthtime":"60368401", "lastagecheckage":"1-0-1972"}
    gameAppPage = session.get(link, cookies=cookies)
    if gameAppPage.url != link :
        # game removed?
        return True

    earlyAccessElem = gameAppPage.html.find("div[class='early_access_header']", first=True)
    if earlyAccessElem != None :
        # skip early access games - give them a chance
        return False

    releaseDateElem = gameAppPage.html.find('div[class="date"]', first=True)
    if releaseDateElem == None :
        # game not released yet - skip
        return False

    try:
        releaseDate = datetime.datetime.strptime(releaseDateElem.text, "%d %b, %Y")
    except ValueError:
        # no release date - skip
        return False

    now = datetime.datetime.now()
    if (now - releaseDate < datetime.timedelta(days=365)):
        # very fresh game to decide
        return False

    ratingValue = gameAppPage.html.find('meta[itemprop="ratingValue"]', first=True)
    if ratingValue == None :
        # game not yet released or do not have rating (too less user reviews)
        return False

    ratingValueInt = int(ratingValue.attrs["content"])
    if (ratingValueInt == 10) :
        # skip - this is cool game
        wishlistGames.append(link)
        return False

    if (ratingValueInt <= 7 and ratingValueInt != 0) :
        # very suspicios game - need to check
        return True

    # check here for steamdb rating?
    steamDbPage = "https://steamdb.info/app/{}/"
    # there is games with very positive and positive rating and released less than year ago and not in EA
    link = steamDbPage.format(app)
    dbGamePage = session.get(link)
    if dbGamePage.url != link :
        # something strange - check page
        return True

    ratingValue = dbGamePage.html.find('meta[itemprop="ratingValue"]', first=True)
    if ratingValue == None :
        # game not yet released or do not have rating (too many user reviews)
        return False

    ratingValueFloat = float(ratingValue.attrs["content"])
    if (ratingValueFloat < 83) :
        # check this game
        return True
    elif (ratingValueFloat > 93) :
        # possible wishlist game?
        wishlistGames.append(link)

    return False

def main(argv) :
  global currentItem
  global totalItems

  global wishlistGames

  print ("Start")

  # some defines
  steamFollowedGamesPage = "https://steamcommunity.com/id/{}/followedgames/"

  parser = argparse.ArgumentParser()
  parser.add_argument("profile", help="steam profile for parse followed games")
  args = parser.parse_args()
  print ("Profile '{}'".format(args.profile))

  print ("Parse Steam Followed Games List")
  session = HTMLSession()
  r = session.get(steamFollowedGamesPage.format(args.profile))
  
  pattern = r'\d{1,8}'
  aaa = {}
  for ddd in r.html.find("div[class='gameListRowItemName']") :
      lll = ddd.find("a", first=True)
      app = re.search(pattern, lll.attrs["href"])[0]
      aaa[app] = lll.text

  print ("Found {} games".format(len(aaa)))

  currentItem = 0
  totalItems = len(aaa)
  printProgressBar(currentItem, totalItems)

  # single thread solution
  gamesToCheck = {steamGamePage.format(app):name for app,name in aaa.items() if parseGamePage(session, app)}

  # print result
  print ("Check these games to remove from followed games")
  for link,name in gamesToCheck.items() :
    print ("{}\t\t{}".format(link, name))

  print ("\nCheck these games to promote from followed to wishlist")
  for link in wishlistGames :
    print ("{}".format(link))

if __name__ == '__main__' :
    main(sys.argv[1:])
