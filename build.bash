#! /bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"

go build -i -o sync "$DIR/sync.go" #gosetup
rc=$?; if [[ $rc != 0 ]]; then exit $rc; fi

cp "$DIR/credentials.json" credentials.json
rc=$?; if [[ $rc != 0 ]]; then exit $rc; fi

tar -cvzpf sync.tar.gz sync credentials.json
rc=$?; if [[ $rc != 0 ]]; then exit $rc; fi

gpg2 --encrypt --sign -r DCB47525 sync.tar.gz
rc=$?; if [[ $rc != 0 ]]; then exit $rc; fi