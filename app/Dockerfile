# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/notiflex-api .

FROM scratch
COPY --from=build /out/notiflex-api /notiflex-api
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/notiflex-api"]
