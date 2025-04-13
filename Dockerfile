FROM golang:1.21 as builder

WORKDIR /app

# Only copy go.mod and go.sum first
COPY go.mod go.sum ./
RUN go mod download

# Now copy rest of the code
COPY . .

RUN CGO_ENABLED=0 go build -o main .
