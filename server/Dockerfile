# 多阶段构建：纯 Go（glebarez/sqlite 无需 cgo）
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/anban ./cmd/anban

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/anban /app/anban
EXPOSE 8090
ENTRYPOINT ["/app/anban"]
