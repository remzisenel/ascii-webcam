package main

import (
	"fmt"
	"image"
	"log"
	"os"

	"github.com/gdamore/tcell"
	"github.com/google/uuid"
	"gocv.io/x/gocv"
)

const (
	logHeight = 1
)

var (
	colorEnabled = false
	runes        = []rune{' ', ' ', ' ', ' ', '.', ',', ':', ';', '+', '*', '?', '%', 'S', '#', '@'}
)

func main() {
	webcam, err := gocv.VideoCaptureDevice(0)
	if err != nil {
		log.Fatalf("Error opening capture device: %v", err)
	}
	defer webcam.Close()

	// Create screen
	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("Error creating screen: %v", err)
	}
	if err := s.Init(); err != nil {
		log.Fatalf("Error initializing screen: %v", err)
	}

	// Set default text style
	// defStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
	defStyle := tcell.StyleDefault
	s.SetStyle(defStyle)

	s.Clear()

	eventChan := make(chan event)
	imageChan := make(chan image.Image)

	go eventListener(s, eventChan)
	go webcamReader(webcam, s, imageChan)

	var lastImage *image.Image
	for {
		select {
		case ev := <-eventChan:
			switch ev {
			case resize:
				logMessage(s, "Resize Requested")
				s.Sync()
			case screenshot:
				filename, err := dumpImageToFile(*lastImage)
				if err != nil {
					logMessage(s, fmt.Sprintf("Error dumping image to file: %v", err))
				} else {
					logMessage(s, fmt.Sprintf("Screenshot saved to file: %v", filename))
				}
			case colorToggle:
				logMessage(s, "Color Toggle")
				colorEnabled = !colorEnabled
			case increaseBrightness:
				logMessage(s, "Increase Brightness")
				if runes[0] == ' ' {
					runes = runes[1:]
				}
			case decreaseBrightness:
				logMessage(s, "Decrease Brightness")
				runes = append([]rune{' '}, runes...)
			case quit:
				s.Fini()
				os.Exit(0)
			}
		case img := <-imageChan:
			width, height := img.Bounds().Dx(), img.Bounds().Dy()
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					pixelColor := img.At(x, y)
					r, g, b, _ := pixelColor.RGBA()
					brightness := float32(r)/0xffff*0.299 + float32(g)/0xffff*0.587 + float32(b)/0xffff*0.114
					runeIndex := int(float32(len(runes)-1) * brightness)

					if colorEnabled {
						color := tcell.NewRGBColor(int32(r), int32(g), int32(b))
						s.SetContent(x, y, runes[runeIndex], nil, defStyle.Foreground(color))
					} else {
						s.SetContent(x, y, runes[runeIndex], nil, defStyle)
					}
				}
			}
			s.Sync()
			lastImage = &img
		}
	}
}

func dumpImageToFile(img image.Image) (string, error) {
	if img == nil {
		return "", fmt.Errorf("image is nil")
	}

	uuid := uuid.New()
	filename := fmt.Sprintf("screenshot-%v.txt", uuid)

	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}

	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelColor := img.At(x, y)
			r, g, b, _ := pixelColor.RGBA()
			brightness := float32(r)/0xffff*0.299 + float32(g)/0xffff*0.587 + float32(b)/0xffff*0.114
			runeIndex := int(float32(len(runes)-1) * brightness)

			file.Write([]byte(string(runes[runeIndex])))
		}
		file.Write([]byte("\n"))
	}

	if err := file.Close(); err != nil {
		log.Fatalf("Error closing file: %v", err)
	}

	return filename, nil
}

func logMessage(s tcell.Screen, message string) {
	s.Clear()

	width, height := s.Size()

	baseY := height - logHeight

	for i := 0; i < len(message); i++ {
		yOffset := i / width
		y := baseY + yOffset

		x := i % width
		s.SetContent(x, y, rune(message[i]), nil, tcell.StyleDefault.Foreground(tcell.ColorRed))
	}
	s.Sync()
}

func webcamReader(webcam *gocv.VideoCapture, s tcell.Screen, imageChan chan<- image.Image) {
	img := gocv.NewMat()
	defer img.Close()

	small := gocv.NewMat()
	defer small.Close()

	for {
		webcam.Read(&img)

		targetWidth, targetHeight := s.Size()

		targetHeight -= logHeight

		gocv.Resize(img, &small, image.Point{
			X: targetWidth,
			Y: targetHeight,
		}, 0, 0, gocv.InterpolationLinear)

		smallImage, err := small.ToImage()
		if err != nil {
			continue
		}

		imageChan <- smallImage
	}
}

func eventListener(s tcell.Screen, eventChan chan<- event) {
	for {
		// Poll event
		ev := s.PollEvent()

		// Process event
		switch ev := ev.(type) {
		case *tcell.EventResize:
			eventChan <- resize
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				eventChan <- quit
			} else if ev.Key() == tcell.KeyRune && ev.Rune() == 'q' {
				eventChan <- quit
			} else if ev.Key() == tcell.KeyRune && ev.Rune() == 'c' {
				eventChan <- colorToggle
			} else if ev.Key() == tcell.KeyRune && ev.Rune() == 's' {
				eventChan <- screenshot
			} else if ev.Key() == tcell.KeyRune && ev.Rune() == '+' {
				eventChan <- increaseBrightness
			} else if ev.Key() == tcell.KeyRune && ev.Rune() == '-' {
				eventChan <- decreaseBrightness
			}
		}
	}
}

type event int

const (
	resize event = iota
	colorToggle
	increaseBrightness
	decreaseBrightness
	screenshot
	quit
)
