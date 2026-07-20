# Multi-stage build shared by all three services. Pick the target with the
# SERVICE build-arg, e.g. `--build-arg SERVICE=http-gateway`.
FROM golang:1.25 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG SERVICE
RUN test -n "$SERVICE" || (echo "SERVICE build-arg is required" && false)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
