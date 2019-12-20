.PHONY: base docker run build static dev run-dev forked again active start cluster prep bash q demo kill watch moar try goversion ubuntu-dev dqlite-dev dqdev dtest  v

VERSION = $(shell date '+%Y%m%d.%H:%M:%S')
RELEASE = xenial
IMG     = paulstuart/dqlited:$(RELEASE)
GIT     = /root/go/src/github.com
SELF    = $(GIT)/paulstuart/dqlited
MNT     = $(GIT)/paulstuart/dqlited
CMN	= /Users/paul.stuart/CODE/DQLITE
DQL	= $(CMN)/src/paulstuart/dqlite
FRK	= $(CMN)/debian/Xenial/FORK

build:
	CGO_LDFLAGS="-L/usr/local/lib -Wl,-rpath=/usr/local/lib" go build -v -tags libsqlite3 -ldflags '-X main.version=$(VERSION)'

v:
	@echo $(VERSION)

static:
	CGO_LDFLAGS="-L/usr/local/lib -Wl,-lco,-ldqlite,-lm,-lraft,-lsqlite3,-luv" go build -tags libsqlite3 -ldflags '-s -w -extldflags "-static"  -X main.version=$(VERSION)'

demo: kill watch start prep moar
	

.PHONY: ubuntu debug

ubuntu:
	@docker build -t paulstuart/ubuntu-base:$(RELEASE) -f docker/Dockerfile.ubuntu .

ubuntu-dev:
	@docker build -t paulstuart/ubuntu-dev:$(RELEASE) -f docker/Dockerfile.dev .

dqlite-dev:
	@docker build -t paulstuart/dqlite-dev:$(RELEASE) -f docker/Dockerfile.dqlite .

debug:
	@docker build --no-cache -t paulstuart/dqlite-debug:$(RELEASE) -f docker/Dockerfile.debug ..

.PHONY: down up ps top

up:
	@docker-compose -p dqlited -f docker/docker-compose.yml up -d

down:
	@docker-compose -p dqlited -f docker/docker-compose.yml down

ps:
	@docker-compose -p dqlited -f docker/docker-compose.yml ps

top:
	@docker-compose -p dqlited -f docker/docker-compose.yml top

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

cluster:
	@scripts/exec.sh ./dqlited cluster

prep:
	@scripts/prep.sh

base:
	docker build -t paulstuart/dqlite-base:$(RELEASE) -f Dockerfile.base .

dev:
	docker build -t paulstuart/dqlite-dev:$(RELEASE) -f Dockerfile.dev .

docker:
	docker build --build-arg release=$(RELEASE) -t $(IMG) .

dtest:
	docker build --build-arg release=$(RELEASE) -t $(IMG) -f Dockerfile.test .

try:
	docker run \
		-it --rm \
		$(IMG) bash

run:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MNT) \
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 						\
                --mount type=bind,src="$(FRK)/go-dqlite",dst=/root/go/src/github.com/canonical/go-dqlite 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) \
		$(IMG) bash

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
		$(IMG) bash
