FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/claim-watcher ./cmd/claim-watcher

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/claim-watcher /claim-watcher

USER nonroot:nonroot
ENTRYPOINT ["/claim-watcher"]
