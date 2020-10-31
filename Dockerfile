# builder
FROM golang:1.14.4-alpine3.12 as builder

ENV GOFLAGS=-mod=readonly
ENV GOCACHE=/tmp/.cache/go-build

COPY ./ $GOPATH/src/github.com/KosukeOhmura/room_crawler

RUN cd $GOPATH/src/github.com/KosukeOhmura/room_crawler \
  && go mod download \
  && go build -o room_crawler


# for production

FROM alpine:3.12

COPY --from=builder /go/src/github.com/KosukeOhmura/room_crawler/room_crawler /room_crawler
ENTRYPOINT ["/room_crawler"]
