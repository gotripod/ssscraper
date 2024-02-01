FROM docker.io/golang@sha256:5c7c2c9f1a930f937a539ff66587b6947890079470921d62ef1a6ed24395b4b3

RUN apt update
RUN apt install poppler-utils -y

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN CGO_ENABLED=0 go install -v ./...

CMD ["ssscraper"]