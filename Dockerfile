FROM golang:1.15.8-alpine3.13 AS build
WORKDIR /kwir
COPY . ./
RUN go build .

FROM alpine:3.13
COPY --from=build /kwir/kwir /
WORKDIR /
CMD ["./kwir","run"]
