.PHONY: base help build static dev run-dev again active start prep bash q demo kill watch moar goversion ubuntu-dev

VERSION = $(shell date '+%Y%m%d.%H:%M:%S') # version our executable with a timestamp (for now)
RELEASE = xenial
IMG     = paulstuart/dqlited:$(RELEASE)
GIT     = /root/go/src/github.com
MNT     = $(GIT)/paulstuart/dqlited
CMN	= /Users/paul.stuart/CODE/DQLITE
DQL	= $(CMN)/src/paulstuart/dqlite
FRK	= $(CMN)/debian/Xenial/FORK

DQLITED_CLUSTER =? "dqlbox1:9181,dqlbox2:9182,dqlbox3:9183,dqlbox4:9184,dqlbox5:9185"

help:	## this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build:	## build the server executable
	CGO_LDFLAGS="-L/usr/local/lib -Wl,-rpath=/usr/local/lib" go build -v -tags libsqlite3 -ldflags '-X main.version=$(VERSION)'

static:	## build a statically linked binary
	CGO_LDFLAGS="-L/usr/local/lib -Wl,-lco,-ldqlite,-lm,-lraft,-lsqlite3,-luv" go build -tags libsqlite3 -ldflags '-s -w -extldflags "-static"  -X main.version=$(VERSION)'

demo: kill watch start prep moar ## demonstrate the cluster bring up and fault tolerance
	
# docker build targets

.PHONY: ubuntu debug docker dqlite-dev dqdev dtest

ubuntu:
	@docker build -t paulstuart/ubuntu-base:$(RELEASE) -f docker/Dockerfile.ubuntu .

ubuntu-dev:
	@docker build -t paulstuart/ubuntu-dev:$(RELEASE) -f docker/Dockerfile.dev .

dqlite-dev:
	@docker build -t paulstuart/dqlite-dev:$(RELEASE) -f docker/Dockerfile.dqlite .

debug:
	@docker build --no-cache -t paulstuart/dqlite-debug:$(RELEASE) -f docker/Dockerfile.debug ..

base:
	docker build -t paulstuart/dqlite-base:$(RELEASE) -f Dockerfile.base .

dev:
	docker build -t paulstuart/dqlite-dev:$(RELEASE) -f Dockerfile.dev .

docker:	## build a "production" image of dqlited
	docker build --build-arg release=$(RELEASE) -t $(IMG) .

dtest:
	docker build --build-arg release=$(RELEASE) -t $(IMG) -f Dockerfile.test .

# our docker-compose targets
.PHONY: down up restart ps top bastion clu bounce status

ID =? 1

bounce:
	@docker-compose -p dqlited -f docker/docker-compose.yml restart dqlbox$(ID)

clu:
	@docker-compose -p dqlited -f docker/docker-compose.yml exec bastion ./dqlited cluster -c $$DQLITE_CLUSTER

up:	## cluster starts
	@docker-compose -p dqlited -f docker/docker-compose.yml up -d

down:	## cluster stops
	@docker-compose -p dqlited -f docker/docker-compose.yml down

restart: down up ## cluster restarts everything

ps:	## show processes
	@docker-compose -p dqlited -f docker/docker-compose.yml ps

top:
	@docker-compose -p dqlited -f docker/docker-compose.yml top

bastion:
	@docker-compose -p dqlited exec bastion bash

kill:
	@pkill dqlited || :

goversion:
	@curl -s -w "\n" https://golang.org/VERSION?m=text

watch:
	@scripts/active.sh -w

moar:
	@scripts/start.sh 4 5

q:
	@./dqlited adhoc "select * from model"

active:
	@scripts/active.sh

bash:
	@scripts/exec.sh bash

start:
	@scripts/start.sh

status: ## show cluster status
	@scripts/exec.sh ./dqlited cluster

prep:
	@scripts/prep.sh

#
# docker targets
#

.PHONEY: run forked try mine

try:
	docker run \
		-it --rm \
		$(IMG) bash

DEVIMG = paulstuart/dqlite-dev:$(RELEASE)

run:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MNT) \
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 						\
                --mount type=bind,src="$(FRK)/go-dqlite",dst=/root/go/src/github.com/canonical/go-dqlite 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) \
		$(IMG) bash


# run docker with my forks mounted over originals
mine:
	@docker run \
		-it --rm \
		--network=dqlite-network				\
		--env DQLITED_CLUSTER="$(DQLITED_CLUSTER)"		\
		--workdir $(MNT) 					\
                --mount type=bind,src="$(DQL)",dst="/opt/build/dqlite" 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) 		\
                --mount type=bind,src="$(PWD)/../FORK/go-dqlite",dst=$(MASTER) 	\
		${DEVIMG} bash

dqdev:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MNT) \
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 						\
                --mount type=bind,src="$(PWD)/../FORK/go-dqlite",dst=$(MASTER) 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) \
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
		paulstuart/dqlited:xenial bash

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
