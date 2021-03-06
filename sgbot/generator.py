import argparse
import sys
import html

def main(argv) :
	parser = argparse.ArgumentParser()
	parser.add_argument("input", help="path to template")
	parser.add_argument("output", help="output path")
	args = parser.parse_args()
	with open(args.input, 'r') as fin:
		read_data = fin.read()

	with open(args.output, 'w') as fout:
		fout.write("""
		package main

		const (
			indexTemplate string = `{}
			{}
			{}`
		)
		""".format('{{define "layout"}}', read_data, '{{end}}'))

if __name__ == '__main__' :
	main(sys.argv[1:])
