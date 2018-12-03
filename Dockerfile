FROM scratch
MAINTAINER Alex Peters <alexander@iov.one>

COPY akms /

ENTRYPOINT ["/akms"]