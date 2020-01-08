#!/bin/bash

# because of docker limitations, we're using Ubuntu Xenial,
# which has an old version of vim in its package repo
# so we'll build the latest from source

# make sure we're really on the assumed release
. /etc/os-release
[[ $UBUNTU_CODENAME == "xenial" ]] || exit

rm -fr /tmp/vim
git clone https://github.com/vim/vim.git /tmp/vim
make -C /tmp/vim
sudo make install -C /tmp/vim
rm -fr /tmp/vim
