FROM releases-docker.jfrog.io/jfrog/jfrog-cli-v2-jf:2.27.1

WORKDIR /root/
RUN mkdir -p .jfrog/plugins/
COPY rt-retention .jfrog/plugins/rt-retention

ENTRYPOINT ["jf", "rt-retention"]
CMD ["--help"]
