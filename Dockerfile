FROM golang:1.23.2

RUN mkdir /app
WORKDIR /app
COPY . /app

EXPOSE 8080