FROM golang:1.21 as build

WORKDIR /app

COPY ./go.mod .
COPY ./go.sum .

RUN go mod download

COPY ./main.go .
COPY ./pkg ./pkg
COPY ./internal ./internal

RUN CGO_ENABLED=0 go build -o proxy main.go

FROM alpine:latest as server

WORKDIR /app

COPY --from=build /app/proxy ./

RUN chmod +x ./proxy

CMD [ "./proxy" ]