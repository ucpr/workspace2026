# Builds one of ./cmd/* selected by the CMD_NAME build-arg.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG CMD_NAME
RUN CGO_ENABLED=0 go build -trimpath -o /out/app ./cmd/${CMD_NAME}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
