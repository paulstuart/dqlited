.PHONY: base docker run build static dev run-dev forked again active start cluster prep bash q demo kill watch moar

RELEASE = xenial
IMG     = paulstuart/dqlited:$(RELEASE)
GIT     = /root/go/src/github.com
SELF    = $(GIT)/paulstuart/dqlited 
MNT     = /root/go/src/github.com/paulstuart/dqlited 
CMN	= /Users/paul.stuart/CODE/DQLITE
DQL	= $(CMN)/src/paulstuart/dqlite

build:
	CGO_LDFLAGS="-L/usr/local/lib -Wl,-rpath=/usr/local/lib" go build -tags libsqlite3

static:
	CGO_LDFLAGS="-L/usr/local/lib -Wl,-lco,-ldqlite,-lm,-lraft,-lsqlite3,-luv" go build -tags libsqlite3 -ldflags '-s -w -extldflags "-static"' 
	@#CGO_LDFLAGS="-L/usr/local/lib -Wl,-lraft,-luv,-lco,-lsqlite3,-ldqlite,-lm" go build -tags libsqlite3 -ldflags '-s -w -extldflags "-static"' 

demo: kill watch start prep moar
	

kill:
	@pkill dqlited || :

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

run:
	docker run \
		-it --rm \
		-p 4001:4001 \
		--workdir $(MNT) \
                --mount type=bind,src="$(DQL)",dst=/opt/build/dqlite 						\
                --mount type=bind,src="$(CMN)/go-dqlite",dst=/root/go/src/github.com/canonical/go-dqlite 	\
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
                --mount type=bind,src="~/CODE/DQLITE/go-dqlite",dst=/root/go/src/github.com/canonical/go-dqlite 	\
                --mount type=bind,src="$(PWD)",dst=$(MNT) \
		$(IMG) bash
