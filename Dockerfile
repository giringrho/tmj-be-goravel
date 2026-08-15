FROM golang:alpine AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0

WORKDIR /build
COPY . .
RUN go mod tidy
RUN go build --ldflags "-s -w -extldflags -static" -o main .

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /www

COPY --from=builder /build/main /www/
COPY --from=builder /build/entrypoint.sh /www/entrypoint.sh
COPY --from=builder /build/.env.example /www/.env.example
COPY --from=builder /build/public/ /www/public/
COPY --from=builder /build/resources/ /www/resources/
COPY --from=builder /build/database/ /www/database/
COPY --from=builder /build/docs/ /www/docs/

RUN chmod +x /www/entrypoint.sh

# Render sets PORT; Goravel reads APP_PORT. Entrypoint maps PORT→APP_PORT.
EXPOSE 3000

ENTRYPOINT ["/www/entrypoint.sh"]
