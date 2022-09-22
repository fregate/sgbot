FROM golang:alpine

RUN apk --update --no-cache add python3

WORKDIR /src

COPY . .

WORKDIR /src/sgbot

RUN go get
RUN go generate
RUN go build

ENTRYPOINT ["/src/sgbot/sgbot"]