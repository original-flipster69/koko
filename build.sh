#!/bin/sh
set -e
go build -buildvcs=false
goda graph -cluster "github.com/original-flipster69/koko/..." | dot -Tsvg -o ./docs/deps.svg
