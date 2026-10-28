FROM golang:1.24-alpine AS build
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/omnirack-server ./cmd/server

FROM alpine:3.21
RUN adduser -D -H appuser
WORKDIR /app
COPY --from=build /out/omnirack-server /app/omnirack-server
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/omnirack-server"]
