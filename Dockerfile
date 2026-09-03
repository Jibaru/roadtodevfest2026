FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /agentarena ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=build /agentarena /agentarena
EXPOSE 8080
ENTRYPOINT ["/agentarena"]
