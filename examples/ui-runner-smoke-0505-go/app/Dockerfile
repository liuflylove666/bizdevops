FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/ui-runner-smoke-0505 .

FROM scratch

COPY --from=builder /out/ui-runner-smoke-0505 /ui-runner-smoke-0505
EXPOSE 8080
ENTRYPOINT ["/ui-runner-smoke-0505"]
