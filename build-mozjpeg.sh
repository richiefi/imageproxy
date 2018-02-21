#!/bin/ash
set -x
git clone https://github.com/mozilla/mozjpeg.git
cd mozjpeg
git checkout v3.3.1
autoreconf -fiv
./configure --prefix="/usr/local/" --libdir="/usr/local/lib64/"
make -j4
make install
