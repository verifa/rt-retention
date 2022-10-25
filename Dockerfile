FROM docker.io/library/golang:1.19.2-alpine3.16 AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

ADD . .
RUN go build -o build/rt-retention


FROM releases-docker.jfrog.io/jfrog/jfrog-cli-v2-jf:2.27.1

WORKDIR /root/
RUN mkdir -p .jfrog/plugins/
COPY --from=build /app/build/rt-retention .jfrog/plugins

CMD ["jf", "rt-retention", "--help"]
