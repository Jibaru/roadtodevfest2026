FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /rapbattle ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=build /rapbattle /rapbattle
EXPOSE 8080
ENTRYPOINT ["/rapbattle"]
