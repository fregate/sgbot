#!/bin/sh

cd sgbot

D=$(date '+%F_%H-%M-%S')
zip ../sgbot-$D.zip bot-func.go thebot.go go.mod func-response.go
