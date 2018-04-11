#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=2

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    setupConode
    test Build
    stopTest
}

testBuild(){
    cp co1/public.toml .
    testOK dbgRun runCo 1 --help
}

main
