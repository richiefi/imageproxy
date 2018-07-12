# imageproxy

Forked from [github.com/willnorris/imageproxy](https://github.com/willnorris/imageproxy/).

imageproxy is a caching image proxy server written in go.  It features:

 - basic image adjustments like resizing, cropping, and rotation
 - access control using host whitelists or request signing (HMAC-SHA256)
 - support for jpeg, png, webp (decode only), tiff, and gif image formats
   (including animated gifs)
 - caching in-memory, on disk, or with Amazon S3, Google Cloud Storage, Azure
   Storage, or Redis

Personally, I use it primarily to dynamically resize images hosted on my own
site (read more in [this post][]).  But you can also enable request signing and
use it as an SSL proxy for remote images, similar to [atmos/camo][] but with
additional image adjustment options.

[this post]: https://willnorris.com/2014/01/a-self-hosted-alternative-to-jetpacks-photon-service
[atmos/camo]: https://github.com/atmos/camo


## URL Structure ##

imageproxy URLs are of the form `http://localhost/{options}/{remote_url}`.

### Options ###

Options are available for cropping, resizing, rotation, flipping, and digital
signatures among a few others.  Options for are specified as a comma delimited
list of parameters, which can be supplied in any order.  Duplicate parameters
overwrite previous values.

The options are specified in query string unlike in the trunk version, for compatibility
with the previously used image processing service.

### Remote URL ###

The URL of the original image to load is specified as the remainder of the
path, without any encoding.  For example,
`http://localhost/200/https://willnorris.com/logo.jpg`.

In order to [optimize caching][], it is recommended that URLs not contain query
strings.

[optimize caching]: http://www.stevesouders.com/blog/2008/08/23/revving-filenames-dont-use-querystring/

### Examples ###

The following live examples demonstrate setting different options on [this
source image][small-things], which measures 1024 by 678 pixels.

[small-things]: https://willnorris.com/2013/12/small-things.jpg

Options                | Meaning                                  | Image
-----------------------|------------------------------------------|------
width=200              | 200px wide, proportional height          | <a href="https://willnorris.com/api/imageproxy/200x/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/200x/https://willnorris.com/2013/12/small-things.jpg" alt="200x"></a>
height=0.15            | 15% original height, proportional width  | <a href="https://willnorris.com/api/imageproxy/x0.15/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/x0.15/https://willnorris.com/2013/12/small-things.jpg" alt="x0.15"></a>
width=100&height=150   | 100 by 150 pixels, cropping as needed    | <a href="https://willnorris.com/api/imageproxy/100x150/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/100x150/https://willnorris.com/2013/12/small-things.jpg" alt="100x150"></a>
size=100               | 100px square, cropping as needed         | <a href="https://willnorris.com/api/imageproxy/100/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/100/https://willnorris.com/2013/12/small-things.jpg" alt="100"></a>
size=150,mode=fit      | scale to fit 150px square, no cropping   | <a href="https://willnorris.com/api/imageproxy/150,fit/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/150,fit/https://willnorris.com/2013/12/small-things.jpg" alt="150,fit"></a>
size=100,rotate=90     | 100px square, rotated 90 degrees         | <a href="https://willnorris.com/api/imageproxy/100,r90/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/100,r90/https://willnorris.com/2013/12/small-things.jpg" alt="100,r90"></a>
size=100,flip=v,flip=h | 100px square, flipped horizontal and vertical | <a href="https://willnorris.com/api/imageproxy/100,fv,fh/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/100,fv,fh/https://willnorris.com/2013/12/small-things.jpg" alt="100,fv,fh"></a>
width=200,quality=60   | 200px wide, proportional height, 60% quality | <a href="https://willnorris.com/api/imageproxy/200x,q60/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/200x,q60/https://willnorris.com/2013/12/small-things.jpg" alt="200x,q60"></a>
width=200,format=png   | 200px wide, converted to PNG format | <a href="https://willnorris.com/api/imageproxy/200x,png/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/200x,png/https://willnorris.com/2013/12/small-things.jpg" alt="200x,png"></a>
crop=175,400,300,100   | crop to 400x300px starting at (175,0), scale to 100px wide | <a href="https://willnorris.com/api/imageproxy/cx175,cw400,ch300,100x/https://willnorris.com/2013/12/small-things.jpg"><img src="https://willnorris.com/api/imageproxy/cx175,cw400,ch300,100x/https://willnorris.com/2013/12/small-things.jpg" alt="cx175,cw400,ch300,100x"></a>

Transformation also works on animated gifs.  Here is [this source
image][material-animation] resized to 200px square and rotated 270 degrees:

[material-animation]: https://willnorris.com/2015/05/material-animations.gif

<a href="https://willnorris.com/api/imageproxy/200,r270/https://willnorris.com/2015/05/material-animations.gif"><img src="https://willnorris.com/api/imageproxy/200,r270/https://willnorris.com/2015/05/material-animations.gif" alt="200,r270"></a>

The smart crop feature can best be seen by comparing the following images, with and without smart crop.

<a href="https://willnorris.com/api/imageproxy/150x300,sc/https://judahnorris.com/images/judah-sheets.jpg"><img src="https://willnorris.com/api/imageproxy/150x300,sc/https://judahnorris.com/images/judah-sheets.jpg" alt="200x400,sc"></a>
<a href="https://willnorris.com/api/imageproxy/150x300/https://judahnorris.com/images/judah-sheets.jpg"><img src="https://willnorris.com/api/imageproxy/150x300/https://judahnorris.com/images/judah-sheets.jpg" alt="200x400"></a>

## Getting Started ##

Install the package using:

    go get github.com/richiefi/imageproxy/cmd/imageproxy

Once installed, ensure `$GOPATH/bin` is in your `$PATH`, then run the proxy
using:

    imageproxy

This will start the proxy on port 8080, without any caching and with no host
whitelist (meaning any remote URL can be proxied).  Test this by navigating to
<http://localhost:8080/https://octodex.github.com/images/codercat.jpg?size=500> and
you should see a 500px square coder octocat.

### Cache ###

By default, the imageproxy command does not cache responses, but caching can be
enabled using the `-cache` flag.  It supports the following values:

 - `memory` - uses an in-memory LRU cache.  By default, this is limited to
   100mb. To customize the size of the cache or the max age for cached items,
   use the format `memory:size:age` where size is measured in mb and age is a
   duration.  For example, `memory:200:4h` will create a 200mb cache that will
   cache items no longer than 4 hours.
 - directory on local disk (e.g. `/tmp/imageproxy`) - will cache images
   on disk
 - s3 URL (e.g. `s3://region/bucket-name/optional-path-prefix`) - will cache
   images on Amazon S3.  This requires either an IAM role and instance profile
   with access to your your bucket or `AWS_ACCESS_KEY_ID` and `AWS_SECRET_KEY`
   environmental variables be set. (Additional methods of loading credentials
   are documented in the [aws-sdk-go session
   package](https://docs.aws.amazon.com/sdk-for-go/api/aws/session/)).
 - gcs URL (e.g. `gcs://bucket-name/optional-path-prefix`) - will cache images
   on Google Cloud Storage. Authentication is documented in Google's
   [Application Default Credentials
   docs](https://cloud.google.com/docs/authentication/production#providing_credentials_to_your_application).
 - azure URL (e.g. `azure://container-name/`) - will cache images on
   Azure Storage.  This requires `AZURESTORAGE_ACCOUNT_NAME` and
   `AZURESTORAGE_ACCESS_KEY` environment variables to bet set.
 - redis URL (e.g. `redis://hostname/`) - will cache images on
   the specified redis host. The full URL syntax is defined by the [redis URI
   registration](https://www.iana.org/assignments/uri-schemes/prov/redis).
   Rather than specify password in the URI, use the `REDIS_PASSWORD`
   environment variable.

For example, to cache files on disk in the `/tmp/imageproxy` directory:

    imageproxy -cache /tmp/imageproxy

Reload the [codercat URL][], and then inspect the contents of
`/tmp/imageproxy`.  Within the subdirectories, there should be two files, one
for the original full-size codercat image, and one for the resized 500px
version.

[codercat URL]: http://localhost:8080/https://octodex.github.com/images/codercat.jpg?size=500

If the `-cache` flag is specified multiple times, multiple caches will be
created in a [tiered fashion][]. Typically this is used to put a smaller and
faster in-memory cache in front of a larger but slower on-disk cache.  For
example, the following will first check an in-memory cache for an image,
followed by a gcs bucket:

    imageproxy -cache memory -cache gcs://my-bucket/

[tiered fashion]: https://godoc.org/github.com/die-net/lrucache/twotier

### Referrer Whitelist ###

You can limit images to only be accessible for certain hosts in the HTTP
referrer header, which can help prevent others from hotlinking to images. It can
be enabled by running:

    imageproxy  -referrers example.com


Reload the [codercat URL][], and you should now get an error message.  You can
specify multiple hosts as a comma separated list, or prefix a host value with
`*.` to allow all sub-domains as well.

### Host whitelist ###

You can limit the remote hosts that the proxy will fetch images from using the
`whitelist` flag.  This is useful, for example, for locking the proxy down to
your own hosts to prevent others from abusing it.  Of course if you want to
support fetching from any host, leave off the whitelist flag.  Try it out by
running:

    imageproxy -whitelist example.com

Reload the [codercat URL][], and you should now get an error message.  You can
specify multiple hosts as a comma separated list, or prefix a host value with
`*.` to allow all sub-domains as well.

### Signed Requests ###

Instead of a host whitelist, you can require that requests be signed.  This is
useful in preventing abuse when you don't have just a static list of hosts you
want to allow.  Signatures are generated using HMAC-SHA256 against the remote
URL, and url-safe base64 encoding the result:

    base64urlencode(hmac.New(sha256, <key>).digest(<remote_url>))

The HMAC key is specified using the `signatureKey` flag.  If this flag
begins with an "@", the remainder of the value is interpreted as a file on disk
which contains the HMAC key.

Try it out by running:

    imageproxy -signatureKey "secret key"

Reload the [codercat URL][], and you should see an error message.  Now load a
[signed codercat URL][] and verify that it loads properly.

[signed codercat URL]: http://localhost:8080/https://octodex.github.com/images/codercat.jpg?size=500,signature=sXyMwWKIC5JPCtlYOQ2f4yMBTqpjtUsfI67Sp7huXIYY%3D

Some simple code samples for generating signatures in various languages can be
found in [URL Signing](https://github.com/willnorris/imageproxy/wiki/URL-signing).

If both a whiltelist and signatureKey are specified, requests can match either.
In other words, requests that match one of the whitelisted hosts don't
necessarily need to be signed, though they can be.

### Default Base URLs ###

You may provide prefixes that map to base URLs by storing a JSON document
in a certain location.

Contents of `https://config.config/config.json`:

	{"/proxy":{"base_url::"https://octodex.github.com","default_options":{}}

imageproxy launch command:

    imageproxy -baseURLConfURL https://config.config/config.json

Then load the codercat image, specified as a URL relative to that base:
<http://localhost:8080/proxy/500/images/codercat.jpg>.  Note that this is not an
effective method to mask the true source of the images being proxied; it is
trivial to discover the base URL being used.  Even when a base URL is
specified, you can always provide the absolute URL of the image to be proxied.

### Scaling beyond original size ###

By default, the imageproxy won't scale images beyond their original size.
However, you can use the `scaleUp` command-line flag to allow this to happen:

    imageproxy -scaleUp true

### WebP and TIFF support ###

Imageproxy can proxy remote webp images, but they will be served in either jpeg
or png format (this is because the golang webp library only supports webp
decoding) if any transformation is requested.  If no format is specified,
imageproxy will use jpeg by default.  If no transformation is requested (for
example, if you are just using imageproxy as an SSL proxy) then the original
webp image will be served as-is without any format conversion.

Because so few browsers support tiff images, they will be converted to jpeg by
default if any transformation is requested. To force encoding as tiff, pass the
"tiff" option. Like webp, tiff images will be served as-is without any format
conversion if no transformation is requested.


Run `imageproxy -help` for a complete list of flags the command accepts.  If
you want to use a different caching implementation, it's probably easiest to
just make a copy of `cmd/imageproxy/main.go` and customize it to fit your
needs... it's a very simple command.

## Deploying ##

In most cases, you can follow the normal procedure for building a deploying any
go application.

This version requires OpenCV and either `libjpeg-turbo` or `mozjpeg`. If `mozjpeg` has been
installed with default settings using `make install` (to `/opt/mozjpeg/`),
the following seems to work for building `imageproxy` and running on Ubuntu:

```
export CGO_CFLAGS="-I/opt/mozjpeg/include/"
export CGO_LDFLAGS="-L/opt/mozjpeg/lib/"
export LD_LIBRARY_PATH="/opt/mozjpeg/lib/"
go install github.com/richiefi/imageproxy/cmd/imageproxy
imageproxy
```

Homebrew on macOS provides `libjpeg-turbo` and `opencv`. They can be used in the following way:
```
export CGO_CPPFLAGS="-I/usr/local/Cellar/opencv/3.4.1_2/include -I/usr/local/Cellar/opencv/3.4.1_2/include/opencv2"
export CGO_CXXFLAGS="--std=c++1z -stdlib=libc++"
export CGO_CFLAGS="-I/usr/local/opt/jpeg-turbo/include"
export CGO_LDFLAGS="-L/usr/local/Cellar/opencv/3.4.1_2/lib -lopencv_core -lopencv_face -lopencv_videoio -lopencv_imgproc -lopencv_highgui -lopencv_imgcodecs -lopencv_objdetect -lopencv_features2d -lopencv_video -lopencv_dnn -lopencv_xfeatures2d -lopencv_plot -lopencv_tracking -L/usr/local/opt/jpeg-turbo/lib"
go install github.com/richiefi/imageproxy/cmd/imageproxy
imageproxy
```
