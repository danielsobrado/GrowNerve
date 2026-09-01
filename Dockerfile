FROM golang:1.25.13-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/grownerve-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/grownerve-api /app/grownerve-api
COPY config /app/config
ENV CONFIG_DIR=/app/config
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/grownerve-api"]
