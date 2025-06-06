FROM golang:1.24.3-alpine3.22
# Install wget, curl, nmap, and other network tools
RUN apk update && apk add --no-cache \
    bash \
    curl \
    wget \
    nmap \
    netcat-openbsd \
    bind-tools \
    iputils \
    tcpdump
RUN mkdir /app
ADD . /app
WORKDIR /app
RUN go build -o main .
CMD ["/app/main"]