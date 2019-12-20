
# gather all the sources required to build dqlite (the library)
# note that the repos will need to be updated after this image is built

git clone https://github.com/canonical/dqlite.git		&
git clone --depth 100 https://github.com/canonical/sqlite.git	&
git clone https://github.com/canonical/libco.git		&
git clone https://github.com/libuv/libuv.git			&
git clone https://github.com/canonical/raft.git			&

wait

