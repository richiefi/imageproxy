#!/usr/bin/env bash

# The function build this way might require setting LD_LIBRARY_PATH=/var/task/lib on Lambda, since mozjpeg claims
# to have ABI compatibility with libjpeg-turbo, which is a lie. Linker may therefore use libjpeg-turbo which is
# available on AWS Linux, apparently as a dependency of aws-cli. Libraries in LD_LIBRARY_PATH have priority over
# others.

export GOROOT="/usr/lib/golang"
export GOPATH="$HOME/go"
export PATH="$PATH:$GOROOT/bin"
export PKG_CONFIG_PATH="$HOME/local/lib64/pkgconfig"
export LD_LIBRARY_PATH="$HOME/local/lib64"
export CGO_CFLAGS="-I$HOME/local/include"
export CGO_CPPFLAGS="-I$HOME/local/include"
export CGO_CXXFLAGS="--std=c++11"
export CGO_LDFLAGS="-L$HOME/local/lib64 -lopencv_core -lopencv_face -lopencv_videoio -lopencv_imgproc -lopencv_highgui -lopencv_imgcodecs -lopencv_objdetect -lopencv_features2d -lopencv_video -lopencv_dnn -lopencv_xfeatures2d -lopencv_plot -lopencv_tracking"

sudo yum -y install make cmake git automake autoconf libtool nasm gcc-c++ golang-bin

rm -Rf $HOME/mozjpeg
cd $HOME
git clone https://github.com/mozilla/mozjpeg.git
cd mozjpeg
git checkout v3.3.1
autoreconf -fiv
./configure --prefix="$HOME/local/" --libdir="$HOME/local/lib64/"
make -j4
make install

rm -Rf $HOME/opencv
mkdir $HOME/opencv
cd $HOME/opencv
wget -O opencv.zip https://github.com/opencv/opencv/archive/3.4.1.zip
unzip opencv.zip
wget -O opencv_contrib.zip https://github.com/opencv/opencv_contrib/archive/3.4.1.zip
unzip opencv_contrib.zip

cd $HOME/opencv/opencv-3.4.1
mkdir build
cd build
cmake -D CMAKE_BUILD_TYPE=RELEASE -D CMAKE_INSTALL_PREFIX=$HOME/local -D OPENCV_EXTRA_MODULES_PATH=$HOME/opencv/opencv_contrib-3.4.1/modules -D BUILD_DOCS=OFF BUILD_EXAMPLES=OFF -D BUILD_TESTS=OFF -D BUILD_PERF_TESTS=OFF -D BUILD_opencv_java=OFF -D BUILD_opencv_python=OFF -D BUILD_opencv_python2=OFF -D BUILD_opencv_python3=OFF ..
make -j4
make install

go install github.com/richiefi/imageproxy/cmd/process-image-lambda

mkdir $HOME/package/
mkdir $HOME/package/lib/
cd $HOME/package/
cp $HOME/go/bin/process-image-lambda ./process-image-lambda
cp $HOME/local/lib64/libopencv_core.so.3.4 lib/
cp $HOME/local/lib64/libopencv_face.so.3.4 lib/
cp $HOME/local/lib64/libopencv_videoio.so.3.4 lib/
cp $HOME/local/lib64/libopencv_imgproc.so.3.4 lib/
cp $HOME/local/lib64/libopencv_highgui.so.3.4 lib/
cp $HOME/local/lib64/libopencv_imgcodecs.so.3.4 lib/
cp $HOME/local/lib64/libopencv_objdetect.so.3.4 lib/
cp $HOME/local/lib64/libopencv_features2d.so.3.4 lib/
cp $HOME/local/lib64/libopencv_video.so.3.4 lib/
cp $HOME/local/lib64/libopencv_dnn.so.3.4 lib/
cp $HOME/local/lib64/libopencv_xfeatures2d.so.3.4 lib/
cp $HOME/local/lib64/libopencv_plot.so.3.4 lib/
cp $HOME/local/lib64/libopencv_tracking.so.3.4 lib/
cp $HOME/local/lib64/libopencv_flann.so.3.4 lib/
cp $HOME/local/lib64/libopencv_ml.so.3.4 lib/
cp $HOME/local/lib64/libopencv_photo.so.3.4 lib/
cp $HOME/local/lib64/libopencv_text.so.3.4 lib/
cp $HOME/local/lib64/libopencv_datasets.so.3.4 lib/
cp $HOME/local/lib64/libopencv_shape.so.3.4 lib/
cp $HOME/local/lib64/libopencv_calib3d.so.3.4 lib/
cp $HOME/local/lib64/libjpeg.so.62 lib/
cp $HOME/local/share/OpenCV/haarcascades/haarcascade_frontalface_default.xml ./haarcascade_frontalface_default.xml

zip -r package.zip *
