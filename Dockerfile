FROM golang:1.16
WORKDIR /app/src
VOLUME /app/build
COPY . .
RUN go mod download -x
RUN go build -o /app/build/
