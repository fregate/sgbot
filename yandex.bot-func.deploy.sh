#!/bin/sh

cd sgbot

D=$(date '+%F_%H-%M-%S')
zip ../bot-$D.zip bot-func.go thebot.go go.mod
