.PHONY: vv base help build static dev run-dev again active start prep bash q demo watch moar ubuntu-dev

VERSION = $(shell date '+%Y%m%d.%H:%M:%S') # version our executable with a timestamp (for now)

# docker mac stable as of 2020/01/07 is kernel 4.9.184, 
# so let's not use bionic until docker updates (or we move to edge)
# bionic uses kernel 4.15.0.74.76
#RELEASE = xenial
RELEASE = bionic

IMG     = paulstuart/dqlited:$(RELEASE)
GIT     = /root/go/src/github.com
MNT     = $(GIT)/paulstuart/dqlited
CMN	= /Users/paul.stuart/CODE/DQLITE
DQL	= $(CMN)/src/paulstuart/dqlite
FRK	= $(CMN)/debian/Xenial/FORK

#DQLITED_CLUSTER =? "dqlbox1:9181,dqlbox2:9182,dqlbox3:9183,dqlbox4:9184,dqlbox5:9185"
#COMPOSER_CLUSTER =? "dqlbox1:9181,dqlbox2:9182,dqlbox3:9183,dqlbox4:9184,dqlbox5:9185"
COMPOSER_CLUSTER =? "dqlbox1:9181,dqlbox2:9182,dqlbox3:9183"

COMPOSE = docker-compose -p dqlited -f docker/docker-compose.yml


help:	## this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

vv:
	@echo "VERSION -$(VERSION)-"

build:	fmt 	## build the server executable
	CGO_LDFLAGS="-L/usr/local/lib -Wl,-rpath=/usr/local/lib" go build -v -tags libsqlite3 -ldflags '-X main.version=$(VERSION)'

static:	## build a statically linked binary
	CGO_LDFLAGS="-L/usr/local/lib -Wl,-lco,-ldqlite,-lm,-lraft,-lsqlite3,-luv" go build -tags libsqlite3 -ldflags '-s -w -extldflags "-static"  -X main.version=$(VERSION)'

.PHONY: kill local redo depends rerun triad tz

tz:
	ln -sf /usr/share/zoneinfo/US/Pacific /etc/localtime

depends: 
	/opt/build/scripts/build_dqlite.sh all

redo:	build kill clean start

triad::	kill watch start
rerun:	kill clean watch start prep moar

local:
	@./local

demo: kill watch start prep ## demonstrate the cluster bring up and fault tolerance
	
# docker build targets

.PHONY: ubuntu debug docker dqlited-dev dqlited-prod dq dtest hey dangling dangle timeout tcp

timeout:
	echo 1 > /proc/sys/net/ipv4/tcp_fin_timeout

tcp:
	cat /proc/sys/net/ipv4/tcp_fin_timeout

dangle:	dangling
dangling:
	@docker rmi -f $(docker images -f "dangling=true" -q)

hey:
	echo RELEASE: $(RELEASE)

DOCKER=docker build -f docker/Dockerfile --build-arg release=$(RELEASE)

ubuntu:
	@$(DOCKER) --target base-os  -t paulstuart/ubuntu-base:$(RELEASE) .

ubuntu-dev: # builds upon ubuntu-base
	@$(DOCKER) --target dev-env   -t paulstuart/ubuntu-dev:$(RELEASE) .

dqlited-dev: # builds upon ubuntu-dev
	@$(DOCKER) --target dqlited-dev  -t paulstuart/dqlite-dev:$(RELEASE) .

dqlited-prod: # builds upon ubuntu-dev (for now, will use ubuntu-base once ready)
	@$(DOCKER) --target dqlited-prod  -t paulstuart/dqlite-prod:$(RELEASE) .

#debug:
#	@docker build --no-cache -t paulstuart/dqlite-debug:$(RELEASE) .

#base:
#	docker build -t paulstuart/dqlite-base:$(RELEASE) -f Dockerfile.base .
#
#dev:
#	docker build -t paulstuart/dqlite-dev:$(RELEASE) -f Dockerfile.dev .

docker:	## build a "production" image of dqlited
	docker build --build-arg release=$(RELEASE) -t $(IMG) .

dtest:
	docker build --build-arg release=$(RELEASE) -t $(IMG) -f Dockerfile.test .

# our docker-compose targets
.PHONY: down up restart ps top bastion clu bounce status comptest d1 d2 net log

log:
	@$(COMPOSE) logs

net:
	@$(COMPOSE) network

comptest:
	@$(COMPOSE) up -d bastion

d1:
	@$(COMPOSE) exec dqlbox1 bash
	@#$(COMPOSE) up -d dqlbox1

d2:
	@$(COMPOSE) exec dqlbox2 bash

ID =? 1

bounce:
	@$(COMPOSE) restart dqlbox$(ID)

clu:
	@$(COMPOSE) exec bastion ./dqlited cluster -c $$DQLITE_CLUSTER

up:	## cluster starts
	@$(COMPOSE) up -d

down:	## cluster stops
	@$(COMPOSE) down

restart: down up ## cluster restarts everything

ps:	## show processes
	@$(COMPOSE) ps

top:
	@$(COMPOSE) top

bastion:
	@$(COMPOSE) exec bastion bash

kill:
	@pkill dqlited || :

.phony: goversion fmt clean

clean:
	rm -rf /tmp/dqlited*

fmt:
	gofmt -s -w *.go

goversion:
	@curl -s -w "\n" https://golang.org/VERSION?m=text

watch:
	@scripts/active.sh -w

moar:
	@DQLITED_ROLE=voter scripts/start.sh 4 5

q:
	@./dqlited adhoc "select * from model"

active:
	@scripts/active.sh

bash:
	@scripts/exec.sh bash

start:
	@scripts/start.sh

status: ## show cluster status
	@scripts/exec.sh ./dqlited status

prep:
	@scripts/prep.sh

# docker targets
#

.PHONY: forked try mine run dqx run-ubuntu

try:
	docker run \
		-it --rm \
		$(IMG) bash

DEVIMG = paulstuart/dqlite-dev:$(RELEASE)

run-ubuntu:
	docker run \
		-it --rm \
		--workdir $(MNT) \
		paulstuart/ubuntu-base:$(RELEASE) bash

run:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MNT) \
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 						\
                --mount type=bind,src="$(FRK)/go-dqlite",dst=/root/go/src/github.com/canonical/go-dqlite 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) \
		$(IMG) bash


DOCKER_CLUSTER = "127.0.0.1:9181,127.0.0.1:9182,127.0.0.1:9183,127.0.0.1:9184,127.0.0.1:9185"
#DOCKER_CLUSTER = "127.0.0.1:9181,127.0.0.1:9182,127.0.0.1:9183"
LOCAL_CLUSTER = "@/tmp/dqlited.1.sock,@/tmp/dqlited.2.sock,@/tmp/dqlited.3.sock"

# run docker with my forks mounted over originals
mine:
	@docker run \
		-it --rm \
		--network=dqlite-network				\
		--env DQLITED_CLUSTER="$(DOCKER_CLUSTER)"		\
		--workdir $(MNT) 					\
                --mount type=bind,src="$(DQL)",dst="/opt/build/dqlite" 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) 		\
                --mount type=bind,src="$(PWD)/../FORK/go-dqlite",dst=$(MASTER) 	\
		${DEVIMG} bash

.PHONY: udev bionic

# temp target for testing canonical/raft on ubuntu bionic
bionic:
	docker run \
		-it --rm \
		--privileged								\
		--workdir $(MNT) 							\
                --mount type=bind,src="$(PWD)",dst=$(MNT) 				\
		ubuntu:bionic bash

# temp target for testing image
udev:
	docker run \
		-it --rm \
		-p 4001:4001 \
		-e DQLITED_CLUSTER=$(DOCKER_CLUSTER)					\
		--privileged								\
		--workdir $(MNT) 							\
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 			\
                --mount type=bind,src="$(PWD)/../FORK/go-dqlite",dst=$(MASTER) 		\
                --mount type=bind,src="$(PWD)",dst=$(MNT) 				\
		paulstuart/ubuntu-dev:$(RELEASE) bash

# dev image with local forks mounted in place of originals
# expose port 6060 to share local go docs
dq:
	docker run \
		-it --rm \
		-p 4001:4001 \
		-p 6060:6060 \
		-e DQLITED_CLUSTER=$(DOCKER_CLUSTER)					\
		--privileged								\
		--workdir $(MNT) 							\
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 			\
                --mount type=bind,src="$(PWD)/../FORK/go-dqlite",dst=$(MASTER) 		\
                --mount type=bind,src="$(PWD)",dst=$(MNT) 				\
		paulstuart/dqlite-dev:$(RELEASE) bash

.PHONY: comp prodtest
# testing image used for composer
comp:
	docker run \
		-it --rm \
		-e DQLITED_CLUSTER=$(DOCKER_CLUSTER)					\
		--privileged								\
		--workdir $(MNT) 							\
		paulstuart/dqlite-dev:$(RELEASE) bash

# testing image used for composer
prodtest:
	docker run \
		-it --rm \
		-e DQLITED_CLUSTER=$(DOCKER_CLUSTER)					\
		--privileged								\
		--workdir $(MNT) 							\
		paulstuart/dqlite-prod:$(RELEASE) bash

dqx:
	docker run \
		-it --rm \
		-p 4001:4001 \
		-e DQLITED_CLUSTER=$(LOCAL_CLUSTER)					\
		--privileged								\
		--workdir $(MNT) 							\
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 			\
                --mount type=bind,src="$(PWD)/../FORK/go-dqlite",dst=$(MASTER) 		\
                --mount type=bind,src="$(PWD)",dst=$(MNT) 				\
		paulstuart/dqlite-dev:$(RELEASE) bash

runX:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MNT) \
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 						\
                --mount type=bind,src="$(CMN)/go-dqlite",dst=/root/go/src/github.com/canonical/go-dqlite 	\
                --mount type=bind,src="$(PWD)/../FORK/go-dqlite",dst=$(MASTER) 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) \
		$(IMG) bash

run-dev:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MNT) \
                --mount type=bind,src="$$PWD",dst=$(MNT) \
		paulstuart/dqlite-dev:$(RELEASE) bash

#
# for testing chaings to a forked copy of github.com/canonical/go-dqlite
#
MASTER = /root/go/src/github.com/canonical/go-dqlite

#DQL = /Users/paul.stuart/CODE/DQLITE/src/dqlite

forked:
	@docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MASTER) 					\
                --mount type=bind,src="$(DQL)",dst="/opt/build/dqlite" 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) 		\
                --mount type=bind,src="$(PWD)/../FORK/go-dqlite",dst=$(MASTER) 	\
		paulstuart/dqlited:$(RELEASE) bash

again:
	@docker exec \
		-it \
		--workdir $(MASTER) 					\
		thirsty_wing bash
edit:
	docker run \
		-it --rm \
		--workdir $(MNT) \
                --mount type=bind,src="$(CMN)/go-dqlite",dst=/root/go/src/github.com/canonical/go-dqlite 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) \
		$(DEVIMG) bash
