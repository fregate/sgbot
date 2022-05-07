#!/bin/sh

cd sgbot

D=$(date '+%F_%H-%M-%S')
zip ../digest-$D.zip digest-func.go go.mod func-response.go
