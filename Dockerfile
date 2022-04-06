FROM golang:1.16.3-alpine

WORKDIR /bms

COPY . /bms

ENV GOPROXY=https://goproxy.cn,direct

RUN go mod tidy && go mod download && go build -o main
