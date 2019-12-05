FROM ubuntu:xenial

RUN apt-get update

RUN apt-get install -y apt-utils software-properties-common && \
	apt-get update

RUN apt-get install -y autoconf automake make libtool gcc

RUN add-apt-repository -y ppa:dqlite/master && \
	apt-get update && \
	apt-get install -y dqlite libdqlite-dev

RUN apt-get install -y net-tools

RUN mkdir /opt/build
WORKDIR /opt/build

# TODO: move this to top when rebuilding fresh
ENV DEBIAN_FRONTEND noninteractive

WORKDIR /usr/local
RUN apt-get install -y curl
RUN curl -kL https://dl.google.com/go/go1.13.4.linux-amd64.tar.gz | tar -xzf -

RUN echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

RUN apt-get install -y 	\
	git 		\
	man 		\
	man-db		\
	manpages 	\
	net-tools	\
	pkg-config 	\
	strace		\
	tcl-dev		\
	vim 

RUN mkdir -p ~/go/bin ~/go/pkg ~/go/src
RUN mkdir -p /root/go/src/github.com/paulstuart #/unitlite

# host version uses ssh, but we don't want that inside docker container
RUN git config --global url."https://github.com/".insteadOf "git@github.com:"

RUN echo hey
# get dependencies (TODO: rethink this after evaluating)
#RUN cd /root/go/src/github.com/paulstuart/unitlite/src && go get -u -v
RUN cd /root/go/src/github.com/paulstuart && git clone https://github.com/paulstuart/unitlite.git

ENV PATH="/usr/local/go/bin:${PATH}"

RUN cd /root/go/src/github.com/paulstuart/unitlite/dqtest && go get -u -v ./...
#RUN cd /root/go/src/github.com/paulstuart/unitlite && ls -lR


