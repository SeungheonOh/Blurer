package main

import (
	"flag"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*#cgo LDFLAGS: -lxcb
#cgo CFLAGS: -w
#include <xcb/xcb.h>
#include <xcb/xproto.h>
#include <string.h>
#include <stdlib.h>

xcb_atom_t GetAtom(xcb_connection_t *conn, char *name){
	xcb_atom_t atom;
	xcb_intern_atom_cookie_t cookie;

	cookie = xcb_intern_atom(conn, 0, strlen(name), name);

	xcb_intern_atom_reply_t *reply = xcb_intern_atom_reply(conn, cookie, NULL);
	if(reply) {
		atom = reply->atom;
		free(reply);
	}
	return atom;
}

int GetWindows(xcb_connection_t *conn) {
	xcb_atom_t atom = GetAtom(conn, "_NET_CLIENT_LIST");
	xcb_get_property_cookie_t prop_cookie;
	prop_cookie = xcb_get_property(conn, 0, xcb_setup_roots_iterator(xcb_get_setup(conn)).data->root, atom, 0, 0, (1 << 32)-1);
	xcb_get_property_reply_t *prop_reply;
	prop_reply = xcb_get_property_reply(conn, prop_cookie, NULL);

	return prop_reply->value_len;
}
*/
import "C"

func MakeGaussian(size int, max float64) []float64 {
	var ret []float64

	for x := float64(-1); x <= 1; x += float64(1 / (float64(size) - 1) * 2) {
		Calc := math.Pow(math.E, -(math.Pi * math.Pow(x, 2)))
		ret = append(ret, Calc)
	}

	ratio := max / 1
	for i, _ := range ret {
		ret[i] = math.Round(ratio * ret[i])
		if ret[i] == 0 {
			ret[i] = 1
		}
	}

	return ret
}

func PutMask(imgIn image.Image, mask []float64) image.Image {
	imgOut := image.NewRGBA(imgIn.Bounds())
	bounds := imgIn.Bounds()
	var img = make([][3]uint32, (bounds.Min.Y-bounds.Max.Y)*(bounds.Min.X-bounds.Max.X))
	for y := 0; y < bounds.Max.Y-bounds.Min.Y; y++ {
		for x := 0; x < bounds.Max.X-bounds.Min.X; x++ {
			r, g, b, _ := imgIn.At(x, y).RGBA()
			colors := [3]uint32{r / 257, g / 257, b / 257}
			img[x+(y*(bounds.Max.X-bounds.Min.X))] = colors
		}
	}

	for y := 0; y < bounds.Max.Y-bounds.Min.Y; y++ {
		for x := 0; x < bounds.Max.X-bounds.Min.X; x++ {
			newR, newB, newG := 0, 0, 0
			sum := 0
			for i := -(len(mask) / 2); i < len(mask)/2; i++ {
				if x+i >= bounds.Max.X || x+i < 0 {
					continue
				}
				sum += int(mask[i+len(mask)/2])
				index := x + i + (y * (bounds.Max.X - bounds.Min.X))
				r, g, b := img[index][0], img[index][1], img[index][2]
				newR += int(mask[i+len(mask)/2]) * int(r)
				newG += int(mask[i+len(mask)/2]) * int(g)
				newB += int(mask[i+len(mask)/2]) * int(b)
			}
			newR /= sum
			newG /= sum
			newB /= sum
			img[x+(y*bounds.Max.X-bounds.Min.X)] = [3]uint32{uint32(newR), uint32(newG), uint32(newB)}
		}
	}
	for y := 0; y < bounds.Max.Y-bounds.Min.Y; y++ {
		for x := 0; x < bounds.Max.X-bounds.Min.X; x++ {
			newR, newB, newG := 0, 0, 0
			sum := 0
			for i := -(len(mask) / 2); i < len(mask)/2; i++ {
				if y+i >= bounds.Max.Y || y+i < 0 {
					continue
				}
				sum += int(mask[i+len(mask)/2])
				index := x + ((y + i) * (bounds.Max.X - bounds.Min.X))
				r, g, b := img[index][0], img[index][1], img[index][2]
				newR += int(mask[i+len(mask)/2]) * int(r)
				newG += int(mask[i+len(mask)/2]) * int(g)
				newB += int(mask[i+len(mask)/2]) * int(b)
			}
			newR /= sum
			newG /= sum
			newB /= sum
			img[x+(y*bounds.Max.X-bounds.Min.X)] = [3]uint32{uint32(newR), uint32(newG), uint32(newB)}
		}
	}

	for y := 0; y < bounds.Max.Y-bounds.Min.Y; y++ {
		for x := 0; x < bounds.Max.X-bounds.Min.X; x++ {
			index := x + (y * (bounds.Max.X - bounds.Min.X))
			imgOut.Set(x, y, color.RGBA{uint8(img[index][0]), uint8(img[index][1]), uint8(img[index][2]), 0xff})
		}
	}

	return imgOut
}

func SetWallpaper(command string, filename string) error {
	s := strings.Fields(command + " " + filename)
	if err := exec.Command(s[0], s[1:]...).Run(); err != nil {
		return err
	}
	return nil
}

func Run(activationMinimum, size, increment int, imagePath, cacheDir, command string) {
	XConn := C.xcb_connect(nil, nil)
	defer C.xcb_disconnect(XConn)
	BlurOrder := make(chan bool)

	// Blurer
	go func() {
		blurStatus := false
		currentBlur := 0
		for {
			blur := <-BlurOrder
			if blur == blurStatus {
				continue
			} else if blur {
				log.Println("Blurring")
				blurStatus = true
				for i := currentBlur + 1; i < size; i++ {
					currentBlur = i
					filename := cacheDir + "/" + strconv.Itoa(currentBlur) + "b" + strconv.Itoa(increment) + "-" + strings.ReplaceAll(imagePath, "/", "") + ".png"
					SetWallpaper(command, filename)
					select {
					case b := <-BlurOrder:
						if b != blurStatus {
							i = size
						}
					case <-time.After(time.Millisecond * 500):
					}
				}
			} else if !blur {
				log.Println("Unblurring")
				blurStatus = false
				for i := currentBlur - 1; i >= 0; i-- {
					currentBlur = i
					filename := cacheDir + "/" + strconv.Itoa(currentBlur) + "b" + strconv.Itoa(increment) + "-" + strings.ReplaceAll(imagePath, "/", "") + ".png"
					SetWallpaper(command, filename)
					select {
					case b := <-BlurOrder:
						if b != blurStatus {
							i = -1
						}
					case <-time.After(time.Millisecond * 500):
					}
				}
			}
		}
	}()
	for {
		if C.GetWindows(XConn) >= C.int(activationMinimum) {
			BlurOrder <- true
		} else {
			BlurOrder <- false
		}
		time.Sleep(time.Millisecond * 500)
	}

}

func CheckCache(imagePath, cacheDir string, size, increment int) bool {
	for i := 0; i < size; i++ {
		_, err := os.Stat(cacheDir + "/" + strconv.Itoa(i) + "b" + strconv.Itoa(increment) + "-" + strings.ReplaceAll(imagePath, "/", "") + ".png")
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func GenerateBlurImages(imagePath, cacheDir string, size, increment int) {
	if CheckCache(imagePath, cacheDir, size, increment) {
		log.Println("Cache found, process aborted")
		return
	}
	data, err := os.Open(imagePath)
	if err != nil {
		log.Fatal("No File/Invalid File")
	}

	imgIn, _, err := image.Decode(data)
	if err != nil {
		log.Fatal("Unable to decode")
	}

	log.Println("Input image :", imagePath, ", ", imgIn.Bounds().Max.X, "x", imgIn.Bounds().Max.Y)

	var BlurCreators sync.WaitGroup
	BlurCreators.Add(size)

	for i := 0; i < size; i++ {
		go func(i int) {
			defer BlurCreators.Done()
			start := time.Now()
			size := i * increment
			if size%2 == 0 || size == 0 {
				size++
			}
			mask := MakeGaussian(size, float64(i*increment))

			if i == 0 {
				mask = []float64{0, 1}
			}

			filename := cacheDir + "/" + strconv.Itoa(i) + "b" + strconv.Itoa(increment) + "-" + strings.ReplaceAll(imagePath, "/", "") + ".png"
			f, _ := os.Create(filename)
			png.Encode(f, PutMask(imgIn, mask))
			log.Println(filename, " processed in ", time.Now().Sub(start), "\n\tMask :", mask)
		}(i)
	}
	BlurCreators.Wait()
}

var (
	InputFile         string
	CacheDir          string
	WallpaperSetter   string
	SampleSize        int
	BlurIncrement     int
	ActivationMinimum int
)

func init() {
	flag.StringVar(&InputFile, "f", "", "Input File")
	flag.StringVar(&InputFile, "file", "", "Input File")

	flag.StringVar(&CacheDir, "c", "", "Cache Directory")
	flag.StringVar(&CacheDir, "cache", "/.cache/blurer", "Cache Directory")

	flag.StringVar(&WallpaperSetter, "w", "", "Wallpaper Setter")
	flag.StringVar(&WallpaperSetter, "setter", "feh --bg-fill", "Wallpaper Setter")

	flag.IntVar(&SampleSize, "s", 0, "Sample Size")
	flag.IntVar(&SampleSize, "size", 3, "Sample Size")

	flag.IntVar(&BlurIncrement, "i", 0, "Increment Value")
	flag.IntVar(&BlurIncrement, "increment", 15, "Increment Value")

	flag.IntVar(&ActivationMinimum, "a", 0, "Activation Minimum")
	flag.IntVar(&ActivationMinimum, "activation", 1, "Activation Minimum")
}

func main() {
	flag.Parse()

	if InputFile == "" {
		flag.PrintDefaults()
		return
	}

	os.MkdirAll(os.Getenv("HOME")+CacheDir, os.ModePerm)
	log.Println("-----Creating Images-----")
	GenerateBlurImages(InputFile, os.Getenv("HOME")+CacheDir, SampleSize, BlurIncrement)
	log.Println("-----Entering Main Loop-----")
	Run(ActivationMinimum, SampleSize, BlurIncrement, InputFile, os.Getenv("HOME")+CacheDir, WallpaperSetter)
}
