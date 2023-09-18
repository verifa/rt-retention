# Build
FROM golang:1.21 AS build-stage

WORKDIR /app

COPY . ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build

# Deploy
FROM releases-docker.jfrog.io/jfrog/jfrog-cli-v2-jf:2.48.0

WORKDIR /root/

RUN mkdir -p .jfrog/plugins
COPY --from=build-stage /app/rt-retention .jfrog/plugins

ENTRYPOINT ["jf", "rt-retention"]
CMD ["--help"]
