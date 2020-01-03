#!/bin/bash

# per https://unit.nginx.org/installation/

curl -sL https://nginx.org/keys/nginx_signing.key | apt-key add -

cat > /etc/apt/sources.list.d/unit.list << EOL
deb https://packages.nginx.org/unit/ubuntu/ xenial unit
deb-src https://packages.nginx.org/unit/ubuntu/ xenial unit
EOL

# should probably put this in base image
apt-get install -y apt-transport-https

apt update

apt-get install -y unit \
	unit-dev unit-go unit-php unit-python2.7 unit-python3.5 

