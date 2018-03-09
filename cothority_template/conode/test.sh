#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=2

. $GOPATH/src/gopkg.in/dedis/onet.v1/app/libtest.sh

main(){
    startTest
    setupConode
    test Build
    stopTest
}

testBuild(){
    testOK dbgRun runCo 1 --help
}

main
