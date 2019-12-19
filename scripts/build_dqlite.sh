#!/usr/bin/env bash

say() { printf "\n$*\n\n"; }

say hello
say bye
exit

libuv() {
say "building libuv"
cd libuv
git checkout v1.34.0 # latest version as of now
sh autogen.sh
./configure && make -j && make install
cd -
}


libco() {
say "building libco"
cd libco
make -j && make install
cd -
}

sqlite() {
say "building sqlite"
cd sqlite
git pull
./configure \
	--enable-editline 	\
	--enable-fts5 		\
	--enable-json1 		\
	--enable-update-limit 	\
	--enable-rtree 		\
	--enable-replication &&	\
	make -j && make install
cd -
}


raft() {
say "building raft"
cd raft
git pull
autoreconf -i
./configure
make -j && make install
cd -
}

dqlite() {
say "building dqlite"
cd dqlite
git pull
autoreconf -i
./configure
make -j && make install
cd -
}

[[ -z $1 ]] && exit -1

case $1 in 
	libco)  libco ;;
	libuv)  libuv ;;
	raft)   raft ;;
	sqlite) sqlite ;;
	dqlite) dqlite ;;
	all) libuv ; libco ; raft ; sqlite ; dqlite;;
esac