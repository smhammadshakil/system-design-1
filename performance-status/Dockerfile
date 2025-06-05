FROM golang:1.24.3-alpine3.22
RUN mkdir /app
ADD . /app
WORKDIR /app
EXPOSE 3001
RUN go build -o main .
CMD ["/app/main"]