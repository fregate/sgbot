import argparse
import sys

def main(argv) :
	from pathlib import Path
	home = str(Path.home())

	parser = argparse.ArgumentParser()
	parser.add_argument("output", help="output path")
	args = parser.parse_args()

	with open(args.output, 'w') as fout:
		fout.write("""
		package main

		const (
			cookiesFileName string = "{0}/.config/sgbot/cookies.json"
			listsFileName   string = "{0}/.config/sgbot/gameslist.json"
			configFileName  string = "{0}/.config/sgbot/config.json"
		)
		""".format(home))

if __name__ == '__main__' :
	main(sys.argv[1:])
