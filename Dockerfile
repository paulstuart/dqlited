ARG release=xenial

FROM paulstuart/dqlite-dev:${release}

RUN apt-get update

# normal usage is to mount this against the version of 
# the repo on the host, which will overwrite this local copy
# regardless, this lets us at least get dependencies when
# building inside the container
RUN mkdir -p /root/go/src/github.com/paulstuart && \
    cd /root/go/src/github.com/paulstuart 	&& \
    git clone https://github.com/paulstuart/dqlited.git

RUN cd /root/go/src/github.com/paulstuart/dqlited && go get -u -v ./... || :

RUN go get -u golang.org/x/lint/golint
