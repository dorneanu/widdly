widdly [![License](http://img.shields.io/:license-gpl3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0.html) [![Pipeline status](https://gitlab.com/opennota/widdly/badges/master/pipeline.svg)](https://gitlab.com/opennota/widdly/commits/master)
======

This is a minimal self-hosted app, written in Go, that can serve as a backend
for a personal [TiddlyWiki](http://tiddlywiki.com/).

## Requirements

Go 1.8+

## Install

    go get -u gitlab.com/opennota/widdly

## Use

Run:

    widdly -http :1337 -p letmein -db /path/to/the/database

- `-http :1337` - listen on port 1337 (by default port 8080 on localhost)
- `-p letmein` - protect by the password (optional); the username will be `widdly`.
- `-db /path/to/the/database` - explicitly specify which file to use for the
  database (by default `widdly.db` in the current directory)

widdly will search for `index.html` in this order:

- next to the executable (in the same directory);
- in the current directory;
- embedded in the executable (to embed, run `zip -9 - index.html | cat >> widdly`).

## Build your own index.html

    git clone https://github.com/Jermolene/TiddlyWiki5
    cd TiddlyWiki5
    node tiddlywiki.js editions/empty --build index

Open `editions/empty/output/index.html` in a browser and install some plugins
(at the very least, the "TiddlyWeb and TiddlySpace components" plugin). You
will be prompted to save the updated index.html.

## Similar projects

For a Google App Engine TiddlyWiki server, look at [rsc/tiddly](https://github.com/rsc/tiddly).
