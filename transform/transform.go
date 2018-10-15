package transform

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"  // register gif format
	_ "image/jpeg" // register jpeg format
	"image/png"
	"io"
	"math"
	"os"

	"github.com/disintegration/imaging"
	"github.com/richiefi/imageproxy/options"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/svkoskin/go-libjpeg/jpeg"
	"github.com/svkoskin/smartcrop"
	smartcropGocv "github.com/svkoskin/smartcrop/gocv"
	smartcropLogger "github.com/svkoskin/smartcrop/logger"
	"github.com/svkoskin/smartcrop/nfnt"
	"golang.org/x/image/tiff"   // register tiff format
	_ "golang.org/x/image/webp" // register webp format
	"willnorris.com/go/gifresize"
)

// default compression quality of resized jpegs
const defaultQuality = 95

// maximum distance into image to look for EXIF tags
const maxExifSize = 1 << 20

// resample filter used when resizing images
var resampleFilter = imaging.Lanczos

// Transform the provided image.  img should contain the raw bytes of an
// encoded image in one of the supported formats (gif, jpeg, or png).  The
// bytes of a similarly encoded image is returned.
func Transform(img []byte, opt options.Options) ([]byte, error) {
	if !opt.Transform() {
		// bail if no transformation was requested
		return img, nil
	}

	// decode image
	m, format, err := image.Decode(bytes.NewReader(img))
	if err != nil {
		return nil, err
	}

	// apply EXIF orientation for jpeg and tiff source images. Read at most
	// up to maxExifSize looking for EXIF tags.
	if format == "jpeg" || format == "tiff" {
		r := io.LimitReader(bytes.NewReader(img), maxExifSize)
		if exifOpt := exifOrientation(r); exifOpt.Transform() {
			m = transformImage(m, exifOpt)
		}
	}

	// encode webp and tiff as jpeg by default
	if format == "tiff" || format == "webp" {
		format = "jpeg"
	}

	if opt.Format != "" {
		format = opt.Format
	}

	// transform and encode image
	buf := new(bytes.Buffer)
	switch format {
	case "gif":
		fn := func(img image.Image) image.Image {
			return transformImage(img, opt)
		}
		err = gifresize.Process(buf, bytes.NewReader(img), fn)
		if err != nil {
			return nil, err
		}
	case "jpeg":
		quality := opt.Quality
		if quality == 0 {
			quality = defaultQuality
		}

		libjpegOptions := jpeg.EncoderOptions{
			Quality:         quality,
			OptimizeCoding:  true,
			ProgressiveMode: true,
		}

		m = transformImage(m, opt)

		err = jpeg.Encode(buf, m, &libjpegOptions)
		if err != nil {
			return nil, err
		}
	case "png":
		m = transformImage(m, opt)
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		err = enc.Encode(buf, m)
		if err != nil {
			return nil, err
		}
	case "tiff":
		m = transformImage(m, opt)
		err = tiff.Encode(buf, m, &tiff.Options{tiff.Deflate, true})
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported format: %v", format)
	}

	return buf.Bytes(), nil
}

// evaluateFloat interprets the option value f. If f is between 0 and 1, it is
// interpreted as a percentage of max, otherwise it is treated as an absolute
// value.  If f is less than 0, 0 is returned.
func evaluateFloat(f float64, max int) int {
	if 0 < f && f < 1 {
		return int(float64(max) * f)
	}
	if f < 0 {
		return 0
	}
	return int(f)
}

// resizeParams determines if the image needs to be resized, and if so, the
// dimensions to resize to.
func resizeParams(m image.Image, opt options.Options) (w, h int, resize bool) {
	// convert percentage width and height values to absolute values
	imgW := m.Bounds().Dx()
	imgH := m.Bounds().Dy()
	w = evaluateFloat(opt.Width, imgW)
	h = evaluateFloat(opt.Height, imgH)

	// never resize larger than the original image unless specifically allowed
	if !opt.ScaleUp {
		if w > imgW {
			w = imgW
		}
		if h > imgH {
			h = imgH
		}
	}

	// if requested width and height match the original, skip resizing
	if (w == imgW || w == 0) && (h == imgH || h == 0) {
		return 0, 0, false
	}

	return w, h, true
}

// cropParams calculates crop rectangle parameters to keep it in image bounds
func cropParams(m image.Image, opt options.Options) image.Rectangle {
	if !opt.SmartCrop && opt.CropX == 0 && opt.CropY == 0 && opt.CropWidth == 0 && opt.CropHeight == 0 {
		return m.Bounds()
	}

	// width and height of image
	imgW := m.Bounds().Dx()
	imgH := m.Bounds().Dy()

	if opt.SmartCrop {
		w := evaluateFloat(opt.Width, imgW)
		h := evaluateFloat(opt.Height, imgH)

		detectors := make([]smartcrop.Detector, 3)

		// First detector shall be a face detector if cascades are provided, or skin detector if not
		cascadeXMLPath := os.Getenv("CASCADE_XML_PATH")
		if cascadeXMLPath != "" {
			detectors[1] = &smartcropGocv.FaceDetector{cascadeXMLPath, &smartcropLogger.Logger{}}
		} else {
			detectors[1] = &smartcrop.SkinDetector{}
		}

		detectors[0] = &smartcrop.EdgeDetector{}
		detectors[2] = &smartcrop.SaturationDetector{}

		cropAnalyzer := smartcrop.NewAnalyzer(nfnt.NewDefaultResizer())
		cropAnalyzer.SetDetectors(detectors)

		r, err := cropAnalyzer.FindBestCrop(m, w, h)
		if err == nil {
			return r
		} else {
			fmt.Printf("Smartcrop returned an error: %s\n", err.Error())
		}
	}

	// top left coordinate of crop
	x0 := evaluateFloat(math.Abs(opt.CropX), imgW)
	if opt.CropX < 0 {
		x0 = imgW - x0 // measure from right
	}
	y0 := evaluateFloat(math.Abs(opt.CropY), imgH)
	if opt.CropY < 0 {
		y0 = imgH - y0 // measure from bottom
	}

	// width and height of crop
	w := evaluateFloat(opt.CropWidth, imgW)
	if w == 0 {
		w = imgW
	}
	h := evaluateFloat(opt.CropHeight, imgH)
	if h == 0 {
		h = imgH
	}

	// bottom right coordinate of crop
	x1 := x0 + w
	if x1 > imgW {
		x1 = imgW
	}
	y1 := y0 + h
	if y1 > imgH {
		y1 = imgH
	}

	return image.Rect(x0, y0, x1, y1)
}

// read EXIF orientation tag from r and adjust opt to orient image correctly.
func exifOrientation(r io.Reader) (opt options.Options) {
	// Exif Orientation Tag values
	// http://sylvana.net/jpegcrop/exif_orientation.html
	const (
		topLeftSide     = 1
		topRightSide    = 2
		bottomRightSide = 3
		bottomLeftSide  = 4
		leftSideTop     = 5
		rightSideTop    = 6
		rightSideBottom = 7
		leftSideBottom  = 8
	)

	ex, err := exif.Decode(r)
	if err != nil {
		return opt
	}
	tag, err := ex.Get(exif.Orientation)
	if err != nil {
		return opt
	}
	orient, err := tag.Int(0)
	if err != nil {
		return opt
	}

	switch orient {
	case topLeftSide:
		// do nothing
	case topRightSide:
		opt.FlipHorizontal = true
	case bottomRightSide:
		opt.Rotate = 180
	case bottomLeftSide:
		opt.FlipVertical = true
	case leftSideTop:
		opt.Rotate = 90
		opt.FlipVertical = true
	case rightSideTop:
		opt.Rotate = -90
	case rightSideBottom:
		opt.Rotate = 90
		opt.FlipHorizontal = true
	case leftSideBottom:
		opt.Rotate = 90
	}
	return opt
}

// transformImage modifies the image m based on the transformations specified
// in opt. The returned images are of type *image.NRGBA regardless the source,
// since the imaging library works that way.
func transformImage(m image.Image, opt options.Options) image.Image {
	// Parse crop and resize parameters before applying any transforms.
	// This is to ensure that any percentage-based values are based off the
	// size of the original image.
	rect := cropParams(m, opt)
	w, h, resize := resizeParams(m, opt)

	// crop if needed
	if !m.Bounds().Eq(rect) {
		m = imaging.Crop(m, rect)
	}
	// resize if needed
	if resize {
		if opt.Fit {
			m = imaging.Fit(m, w, h, resampleFilter)
		} else {
			if w == 0 || h == 0 {
				m = imaging.Resize(m, w, h, resampleFilter)
			} else {
				m = imaging.Thumbnail(m, w, h, resampleFilter)
			}
		}
	}

	// rotate
	rotate := float64(opt.Rotate) - math.Floor(float64(opt.Rotate)/360)*360
	switch rotate {
	case 90:
		m = imaging.Rotate90(m)
	case 180:
		m = imaging.Rotate180(m)
	case 270:
		m = imaging.Rotate270(m)
	}

	// flip
	if opt.FlipVertical {
		m = imaging.FlipV(m)
	}
	if opt.FlipHorizontal {
		m = imaging.FlipH(m)
	}

	return m
}