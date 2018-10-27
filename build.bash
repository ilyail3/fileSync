#! /bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"

. "$DIR/env.bash"

GOPATH="$( realpath $DIR/../../../.. )"

$GOROOT/bin/go build -o ~/.bin/sync "$DIR/sync.go"
rc=$?; if [[ $rc != 0 ]]; then exit $rc; fi

echo "build successful, syncing credentials"

rsync "$DIR/credentials.json" "$DIR/token.json" ~/.bin/
rc=$?; if [[ $rc != 0 ]]; then exit $rc; fi