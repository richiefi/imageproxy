FROM richiefi/opencv-for-gocv:latest as build

WORKDIR /go/src/github.com/richiefi/imageproxy
ADD . .

ENV PKG_CONFIG_PATH /usr/local/lib64/pkgconfig
ENV LD_LIBRARY_PATH /usr/local/lib64
ENV CGO_CPPFLAGS -I/usr/local/include
ENV CGO_CXXFLAGS --std=c++1z
ENV CGO_LDFLAGS -L/usr/local/lib -lopencv_core -lopencv_face -lopencv_videoio -lopencv_imgproc -lopencv_highgui -lopencv_imgcodecs -lopencv_objdetect -lopencv_features2d -lopencv_video -lopencv_dnn -lopencv_xfeatures2d -lopencv_plot -lopencv_tracking

RUN apk add --update autoconf automake build-base libtool nasm git
RUN ./build-mozjpeg.sh
RUN go install -ldflags="-X github.com/richiefi/imageproxy.buildVersion=`date +%s`" github.com/richiefi/imageproxy/cmd/imageproxy

FROM alpine:3.7
RUN apk add --update ca-certificates libstdc++

WORKDIR /go/bin
COPY --from=build /usr/local/share/OpenCV/haarcascades/haarcascade_frontalface_default.xml .
COPY --from=build /usr/local/lib64/ /usr/local/lib64/
COPY --from=build /go/bin/imageproxy .
COPY --from=build /go/src/github.com/richiefi/imageproxy/entrypoint.sh .

ENV LD_LIBRARY_PATH /usr/local/lib64/
ENV CASCADE_XML_PATH ./haarcascade_frontalface_default.xml
ENV PORT 8080
EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
