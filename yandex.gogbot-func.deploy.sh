#!/bin/sh

cd gogbot

D=$(date '+%F_%H-%M-%S')
zip ../gogbot-$D.zip bot-func.go thebot.go go.mod response.go
