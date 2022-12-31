#!/bin/bash

if [ `whoami` == "root" ]; then
	echo "Please do not run this script directly as root. Use your normal development user account."
	exit 1
fi

if [ "`sudo whoami`x" != "rootx" ]; then
	echo "Cannot sudo to root. Please correct (install/configure sudo for your user) and try again."
	exit 1
fi

sudo apt update
# Install g++
sudo apt -y install g++
# Install pcre
sudo apt -y install libpcre3 libpcre3-dev libpcre2-dev
# Download swig (source: http://www.swig.org/download.html)
wget http://prdownloads.sourceforge.net/swig/swig-4.1.1.tar.gz
# Unzip file & cd into directory
chmod a+rx swig-4.1.1.tar.gz && tar -xzvf swig-4.1.1.tar.gz
cd swig-4.1.1
# Specify swig install directory, e.g.:
./configure --prefix=/home/$(whoami)/swigtool
# Compile and install
sudo make
sudo make install
# Add SWIG_PATH environment variable and add it in PATH
export SWIG_PATH=/home/$(whoami)/swigtool/bin
export PATH=$SWIG_PATH:$PATH