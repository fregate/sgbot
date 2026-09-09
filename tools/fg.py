import sys
import argparse
import datetime

import requests

storeFrontApi = "https://store.steampowered.com"
appReviews = "appreviews"
appInfo = "api/appdetails"

steamGamePage = "https://store.steampowered.com/app/{}/"
totalItems = 0
currentItem = 0

wishlistGames = {}
gameNames = {}

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

def parseGamePage(app) :
    global currentItem
    currentItem = currentItem + 1
    printProgressBar(currentItem, totalItems)

    global wishlistGames
    global gameNames

    link = "/".join([storeFrontApi, appInfo])
    r = requests.get(link, params={"appids": app, "json": 1})
    try:
        gameInfo = r.json()[f"{app}"]["data"]
    except (ValueError, KeyError, TypeError):
        print (f"Failed to get {app} info from steam storefront api")
        return False

    gameNames[app] = str(gameInfo["name"])

    # is Coming Soon?
    if gameInfo["release_date"]["date"] == True:
       return False

    try:
        releaseDate = datetime.datetime.strptime(gameInfo["release_date"]["date"], "%d %b, %Y")
    except ValueError:
        # no release date - skip
        return False

    if (datetime.datetime.now() - releaseDate < datetime.timedelta(days=365)):
        # very fresh game to decide
        return False

    # is Early Access?
    ea = next((True for g in gameInfo["genres"] if g["id"] == 70), False)
    if ea == True:
       return False

    # calculate ratings
    link = "/".join([storeFrontApi, appReviews, str(app)])
    r = requests.get(link, params={"num_per_page": 1, "json": 1})
    try:
        reviewScore = r.json()["query_summary"]
    except (ValueError, KeyError, TypeError):
        print (f"Failed to get {app} reviews from steam storefront api")
        return

    totalReviews = reviewScore["total_reviews"]
    if totalReviews < 10:
       return True

    ratingValue = float(reviewScore["total_positive"] / reviewScore["total_reviews"]) * 100
    if (ratingValue < 83.0): # check this game for relegation
        return True
    elif (ratingValue > 93.0): # possible promote to wishlist game?
        wishlistGames[steamGamePage.format(app)] = gameNames.get(app,"")

    return False

def main(argv) :
  global currentItem
  global totalItems

  global wishlistGames
  global gameNames

  print ("Start")

  # some defines
  steamApiGetGamesFollowed = "https://api.steampowered.com/IStoreService/GetGamesFollowed/v1/"

  parser = argparse.ArgumentParser()
  parser.add_argument("profile", help="numeric steam id of the profile to get followed games for")
  parser.add_argument("key", help="steam web api key")
  args = parser.parse_args()
  print ("Profile '{}'".format(args.profile))

  print ("Get Steam Followed Games List")
  r = requests.get(steamApiGetGamesFollowed, params={"id": args.key, "steamid": args.profile})
  try:
    appids = r.json()["response"]["appids"]
  except (ValueError, KeyError, TypeError):
    print ("Failed to get followed games from steam api")
    return

  totalItems = len(appids)
  print ("Found {} games".format(totalItems))

  currentItem = 0
  printProgressBar(currentItem, totalItems)

  # single thread solution
  gamesToCheck = {steamGamePage.format(app):gameNames.get(app,"") for app in appids if parseGamePage(app)}

  # print result
  print ("Check these games to remove from followed games")
  for link,name in gamesToCheck.items() :
    print ("{}\t\t{}".format(link, name))

  print ("\nCheck these games to promote from followed to wishlist")
  for link in wishlistGames :
    print ("{}".format(link))

if __name__ == '__main__' :
    main(sys.argv[1:])
