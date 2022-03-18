#!/bin/bash

ACTION=${1}

function run() {
    export BOOTSTRAP_SERVERS=127.0.0.1:9092
    go run ./server/server.go
}

function clean() {
    rpk topic delete log-servico-canal && rpk topic create log-servico-canal
}

${ACTION}