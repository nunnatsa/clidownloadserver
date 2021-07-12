FROM golang:1.16.4 AS builder

WORKDIR /go/src/github.com/nunnatsa/clidownloadserver
COPY . ./
RUN go test . && \
    go build .

FROM registry.access.redhat.com/ubi8/ubi-minimal
ARG KV_VERSION
ENV USER_UID=1001 \
    USER_NAME=clidownload \
    HOME=/home/clidownload \
    VERSION=${KV_VERSION}
WORKDIR ${HOME}

RUN mkdir -p ${HOME} && \
    chown ${USER_UID}:0 ${HOME} && \
    chmod ug+rwx ${HOME} && \
    # runtime user will need to be able to self-insert in /etc/passwd
    chmod g+rw /etc/passwd

USER ${USER_UID}
ENTRYPOINT ./clidownloadserver
EXPOSE 8080

COPY --from=builder /go/src/github.com/nunnatsa/clidownloadserver/clidownloadserver .
COPY files/ files/
COPY metadata/ metadata/
