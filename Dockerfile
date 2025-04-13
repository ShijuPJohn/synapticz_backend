FROM golang:1.21-alpine AS builder

RUN mkdir -p /myapp
WORKDIR /myapp
COPY . .
RUN CGO_ENABLED=0 go build -o main .

###################
# Package Stage
###################
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /myapp/main /usr/local/bin/main
EXPOSE 5000
CMD ["main"]