FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /aperture .

FROM alpine:3.21

RUN adduser -D -g '' aperture
USER aperture

COPY --from=build /aperture /aperture

EXPOSE 8080 9090
ENV APERTURE_CONFIG=/app/config.yaml

ENTRYPOINT ["/aperture"]
CMD ["--config", "/app/config.yaml"]
