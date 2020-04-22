#!/usr/bin/env bash

say() { printf "\n$*\n\n"; }

libuv() {
say "building libuv"
cd libuv
git pull
git checkout v1.34.2 # latest version as of now
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
rm -f sqlite3 # force rebuild of binary
git pull
./configure \
	--enable-readline	\
	--enable-editline	\
	--enable-fts5		\
	--enable-json1		\
	--enable-update-limit	\
	--enable-rtree		\
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
#CFLAGS=-DDEBUG_VERBOSE=1 
./configure
make clean
make -j CFLAGS=-DDEBUG_VERBOSE=1 && make install
cd -
}

[[ -z $1 ]] && exit -1

cd /opt/build/src

while [[ -n $1 ]]; do
    case $1 in 
	libco)  libco ;;
	libuv)  libuv ;;
	raft)   raft ;;
	sqlite) sqlite ;;
	dqlite) dqlite ;;
	all) libuv ; libco ; raft ; sqlite ; dqlite;;
    esac
    shift
done
