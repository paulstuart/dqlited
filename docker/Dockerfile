
######################################################################################
#
# A miminal but useful setup of ubuntu for building, testing, and hosting
# it builds upon the reference dqlite image to allow to stare and compare
#

# bionic and others seem to have issues on docker mac (stable)
ARG release=xenial

FROM ubuntu:${release} as base-os

ENV DEBIAN_FRONTEND noninteractive

RUN echo "using release: ${release}"

# Do not exclude man pages & other documentation (as of Bionic)
RUN rm -f /etc/dpkg/dpkg.cfg.d/excludes

RUN apt-get update

# make install happy
RUN apt-get install -y apt-utils

# basics for repo mgmt
RUN apt-get install -y 			\
	ca-certificates			\
	software-properties-common 	\
	apt-transport-https &&		\
	apt-get update

# tools that are nice to have whilst logged in
RUN apt-get install -y 	\
	curl		\
	dnsutils 	\
	gdb 		\
	git 		\
	iputils-ping	\
	jq 		\
	lsof 		\
	man 		\
	man-db		\
	manpages 	\
	mercurial 	\
	net-tools	\
	netcat	 	\
	nmap	 	\
	socat	 	\
	sudo	 	\
	strace		\
	telnet		\
	wamerican	\
	vim

######################################################################################
#
# create an image for building dqlite from scratch
# it builds upon the reference dqlite image to allow to stare and compare
#

FROM base-os as dev-env

RUN apt-get update

RUN apt-get install -y 	\
	autoconf 	\
	automake 	\
	gcc		\
	git 		\
	libaio-dev	\
	libncurses-dev	\
	libreadline-dev	\
	libtool 	\
	make 		\
	pkg-config 	\
	tcl-dev

# set up Go
RUN cd /usr/local && \
	curl -skL https://dl.google.com/go/go1.13.5.linux-amd64.tar.gz | tar -xzf -

RUN mkdir -p /root/go/bin /root/go/mod /root/go/pkg /root/go/src
ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH
RUN echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# the build area for C libraries

RUN mkdir /opt/build
WORKDIR /opt/build

# make sure rebuilt libraries supercede installed versions
RUN printf "/usr/local/lib\ninclude /etc/ld.so.conf.d/*.conf\n" > /etc/ld.so.conf && ldconfig

# host mode of git uses ssh, but we don't want that inside docker container
RUN git config --global url."https://github.com/".insteadOf "git@github.com:"

RUN go get -u golang.org/x/lint/golint

# make gdb happier with Go
RUN echo "add-auto-load-safe-path /usr/local/go/src/runtime/runtime-gdb.py" >> /root/.gdbinit

# upgrade vim (if we're still on Xenial
ADD scripts/build_vim.sh /tmp
RUN /tmp/build_vim.sh

# move this up once its working correctly
RUN git clone https://github.com/fatih/vim-go.git ~/.vim/pack/plugins/start/vim-go

# add the vim-go supporting binaries
RUN vim +GoInstallBinaries +qall

# TODO: move this up to base-os
RUN echo "alias vi='vim'" >> ~/.bash_aliases

######################################################################################
#
# create an image for building dqlite from scratch
# it builds upon the reference dqlite image to allow to stare and compare
#

FROM dev-env as dqlited-dev

RUN apt-get update

RUN mkdir -p /opt/build/scripts

ADD scripts/build_dqlite.sh /opt/build/scripts/
ADD scripts/sources.sh /opt/build/scripts/

WORKDIR /opt/build

# gather all the sources required to build dqlite
# note that the repos will need to be updated after this image is built

RUN scripts/sources.sh .

RUN scripts/build_dqlite.sh all

WORKDIR /root/go/src/github.com/paulstuart

RUN git clone https://github.com/paulstuart/dqlited.git

WORKDIR /root/go/src/github.com/paulstuart/dqlited 

RUN go get -u -v ./... || :

RUN make static

RUN go clean -i

RUN echo "export PATH=$PWD:\$PATH" >> ~/.bashrc

######################################################################################
#
# create a production image (but with having nice things for debugging)
#

#FROM base-os as dqlited-prod
# whilst testing
FROM dqlited-dev as dqlited-prod

RUN scripts/sources.sh .

WORKDIR /root/go/src/github.com/paulstuart/dqlited/

#TESTING
COPY . /root/go/src/github.com/paulstuart/dqlited/

#TESTING
RUN make static

# revert to this after testing

#COPY  --from=dqlited-dev /root/go/src/github.com/paulstuart/dqlited/dqlited /usr/local/bin/

#COPY  --from=dqlited-dev /usr/local/bin/sqlite3 /usr/local/bin/
