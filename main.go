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
	labelWidth    = 1024
	labelHeight   = 80
	labelFontSize = 30.0
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
	logoWidth := 200
	logoHeight := 200

	// Resize the logo image while maintaining its aspect ratio
	resizedLogo := imaging.Fit(logoImg, logoWidth, logoHeight, imaging.Lanczos)

	// Add the logo to the center of the QR code
	qrImg := qr.Image(1024)
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

	// Define the background color for the label
	backgroundColor := color.RGBA{R: 1, G: 124, B: 254, A: 255}

	// Create the label image with a background color
	labelImg := image.NewRGBA(image.Rect(0, 0, labelWidth, labelHeight))
	draw.Draw(labelImg, labelImg.Bounds(), &image.Uniform{C: backgroundColor}, image.ZP, draw.Src)

	labelContext := freetype.NewContext()
	labelContext.SetDPI(72)
	labelContext.SetFont(font)
	labelContext.SetFontSize(labelFontSize)
	labelContext.SetClip(labelImg.Bounds())
	labelContext.SetDst(labelImg)
	labelContext.SetSrc(image.White)

	condition := len(labelText) * 2
	// Create the context for drawing text
	labelX := ((labelWidth / 2) - (len(labelText) * 7)) + (len(labelText)-condition)*3
	labelY := labelHeight - int(labelFontSize)

	// Calculate the width of the background fill
	fillWidth := labelWidth

	// Calculate the spacing
	labelAndSpacingHeight := labelHeight

	// Calculate the horizontal position for the background fill
	fillX := (labelWidth - fillWidth) / 2

	// Calculate the vertical position for the background fill
	fillY := qrImg.Bounds().Max.Y - labelAndSpacingHeight - labelHeight

	// Draw the background fill below the label text
	fillRect := image.Rect(fillX, fillY, fillX+fillWidth, qrImg.Bounds().Max.Y)
	draw.Draw(labelImg, fillRect, &image.Uniform{C: backgroundColor}, image.ZP, draw.Src)

	// Set the starting position of the text
	pt := freetype.Pt(labelX, labelY)
	_, err = labelContext.DrawString(labelText, pt)
	if err != nil {
		log.Println("Failed to draw label:", err)
	}

	// Calculate the new height for the qrImg bounds
	newHeight := qrImg.Bounds().Dy() + labelAndSpacingHeight

	// Create a new rectangle with the updated height
	newBounds := image.Rect(qrImg.Bounds().Min.X, qrImg.Bounds().Min.Y, qrImg.Bounds().Max.X, newHeight)

	// Create a new image with the updated bounds
	newQrImg := image.NewRGBA(newBounds)

	// Copy the qrImg to the new image
	draw.Draw(newQrImg, qrImg.Bounds(), qrImg, image.Point{}, draw.Src)

	// Overlay the resized logo on the QR code image with increased spacing
	qrWithLogoAndLabel := imaging.Overlay(newQrImg, labelImg, image.Pt(0, qrImg.Bounds().Dy()), 1.0)

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
	err = imaging.Save(qrWithLogoAndLabel, outputPath)
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
