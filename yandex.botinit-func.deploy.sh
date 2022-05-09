#!/bin/sh

cd sgbot

D=$(date '+%F_%H-%M-%S')
zip ../init-$D.zip bot-init-func.go go.mod func-response.go
