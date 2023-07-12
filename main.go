package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/gorilla/mux"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	tempDir       = "temp"
	logoFile      = "smartlink-logo.png"
	outputFile    = "SmartQR.png"
	labelWidth    = 225
	labelHeight   = 50
	labelFontSize = 15.0
)

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/qrcode", generateQRCode).Methods("GET")
	router.HandleFunc("/qrcode/download", downloadQRCode).Methods("GET")

	log.Fatal(http.ListenAndServe(":8080", router))
}

func generateQRCode(w http.ResponseWriter, r *http.Request) {
	data := r.FormValue("data")
	if data == "" {
		http.Error(w, "Missing 'data' parameter", http.StatusBadRequest)
		return
	}

	logo, err := os.Open(logoFile)
	if err != nil {
		http.Error(w, "Failed to open logo file", http.StatusInternalServerError)
		return
	}

	defer logo.Close()

	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	// Read and resize the logo image
	logoImg, _, err := image.Decode(logo)
	if err != nil {
		http.Error(w, "Failed to decode logo image", http.StatusInternalServerError)
		return
	}
	// Define the desired width and height of the logo image
	logoWidth := 65
	logoHeight := 65

	// Resize the logo image while maintaining its aspect ratio
	resizedLogo := imaging.Fit(logoImg, logoWidth, logoHeight, imaging.Lanczos)

	// Add the logo to the center of the QR code
	qrImg := qr.Image(256)
	if err != nil {
		http.Error(w, "Failed to generate QR code image", http.StatusInternalServerError)
		return
	}
	// Calculate the position to overlay the logo at the center of the QR code
	logoX := (qrImg.Bounds().Max.X - resizedLogo.Bounds().Max.X) / 2
	logoY := (qrImg.Bounds().Max.Y - resizedLogo.Bounds().Max.Y) / 2
	logoPos := image.Point{X: logoX, Y: logoY}

	// Overlay the resized logo on the QR code image
	qrImg = imaging.Overlay(qrImg, resizedLogo, logoPos, 1.0)

	// Load font file
	fontPath := "Roboto-Medium.ttf"
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		http.Error(w, "Failed to load font file", http.StatusInternalServerError)
		return
	}

	font, err := truetype.Parse(fontBytes)
	if err != nil {
		http.Error(w, "Failed to parse font", http.StatusInternalServerError)
		return
	}

	// Retrieve the label text from the form value
	labelText := r.FormValue("label")
	if labelText == "" {
		http.Error(w, "Missing 'label' parameter", http.StatusBadRequest)
		return
	}

	// Create the label image
	labelImg := image.NewRGBA(image.Rect(0, 0, labelWidth, labelHeight))
	draw.Draw(labelImg, labelImg.Bounds(), &image.Uniform{C: color.Transparent}, image.ZP, draw.Src)

	// Create the context for drawing text
	labelX := (labelWidth - len(labelText)*5) / 2
	labelY := labelHeight - int(labelFontSize) + 10

	labelContext := freetype.NewContext()
	labelContext.SetDPI(72)
	labelContext.SetFont(font)
	labelContext.SetFontSize(labelFontSize)
	labelContext.SetClip(labelImg.Bounds())
	labelContext.SetDst(labelImg)
	labelContext.SetSrc(image.Black)

	// Set the starting position of the text
	pt := freetype.Pt(labelX, labelY)
	_, err = labelContext.DrawString(labelText, pt)
	if err != nil {
		log.Println("Failed to draw label:", err)
	}

	// Overlay the resized logo and label on the QR code image
	qrWithLabel := imaging.Overlay(qrImg, labelImg, image.Pt(0, qrImg.Bounds().Max.Y-labelHeight), 1.0)

	// Create a temporary directory if it doesn't exist
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		err := os.Mkdir(tempDir, os.ModePerm)
		if err != nil {
			http.Error(w, "Failed to create temporary directory", http.StatusInternalServerError)
			return
		}
	}

	// Save the QR code image to a temporary file
	outputPath := filepath.Join(tempDir, outputFile)
	err = imaging.Save(qrWithLabel, outputPath)
	if err != nil {
		http.Error(w, "Failed to save QR code image", http.StatusInternalServerError)
		return
	}

	fmt.Println("QR code generated successfully!")

	// Serve the generated QR code image for preview
	http.ServeFile(w, r, outputPath)
}

func downloadQRCode(w http.ResponseWriter, r *http.Request) {
	// Set the appropriate headers for downloading the file
	w.Header().Set("Content-Disposition", "attachment; filename=SmartQR.png")
	w.Header().Set("Content-Type", "image/png")

	// Serve the generated QR code image for download
	outputPath := filepath.Join(tempDir, outputFile)
	http.ServeFile(w, r, outputPath)
}
