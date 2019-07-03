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

## Flat file store

Instead of a bolt database, you can build widdly with a flat file store. Just add `-tags flatfile`
after `go get` or `go build`.

- `-db /path/to/a/directory` - the directory where the data (as ordinary files) will be stored
(by default `widdly_data` in the current directory).

## DynamoDB store

You can also use DynamoDB to store your tiddlers. Before doing this make sure you have a
dedicated account with enough permissions to create/change/delete tables in DynamoDB. In order
to build the binary with support for DynamoDB make sure you add `-tags dynamodb` after `go get` 
or `go build`.

- `-endpoint endpoint-url` - the endpoint URL of your DynamoDB (e.g. https://dynamodb.eu-west-1.amazonaws.com) 

## Build your own index.html

    git clone https://github.com/Jermolene/TiddlyWiki5
    cd TiddlyWiki5
    node tiddlywiki.js editions/empty --build index

Open `editions/empty/output/index.html` in a browser and install some plugins
(at the very least, the "TiddlyWeb and TiddlySpace components" plugin). You
will be prompted to save the updated index.html.

## Similar projects

For a Google App Engine TiddlyWiki server, look at [rsc/tiddly](https://github.com/rsc/tiddly).
