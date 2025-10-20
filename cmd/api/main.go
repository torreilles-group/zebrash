package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ingridhq/zebrash"
	"github.com/ingridhq/zebrash/drawers"
)

func main() {
	port := "3030"
	
	http.HandleFunc("/v1/printers/", handleLabelRequest)
	http.HandleFunc("/health", handleHealth)
	
	log.Printf("Starting Labelary-compatible API server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleLabelRequest(w http.ResponseWriter, r *http.Request) {
	// Parse URL path: /v1/printers/{dpmm}/labels/{width}x{height}/{index}/{zpl}
	// or /v1/printers/{dpmm}/labels/{width}x{height}/{index}/ for POST
	
	path := strings.TrimPrefix(r.URL.Path, "/v1/printers/")
	parts := strings.Split(path, "/")
	
	if len(parts) < 4 {
		http.Error(w, "Invalid URL format. Expected: /v1/printers/{dpmm}/labels/{width}x{height}/{index}/[zpl]", http.StatusBadRequest)
		return
	}
	
	// Parse dpmm
	dpmmStr := strings.TrimSuffix(parts[0], "dpmm")
	dpmm, err := strconv.Atoi(dpmmStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid dpmm value: %s", parts[0]), http.StatusBadRequest)
		return
	}
	
	// Validate "labels" keyword
	if parts[1] != "labels" {
		http.Error(w, "Invalid URL format. Expected 'labels' keyword", http.StatusBadRequest)
		return
	}
	
	// Parse width x height
	dimensions := strings.Split(parts[2], "x")
	if len(dimensions) != 2 {
		http.Error(w, "Invalid dimensions format. Expected: {width}x{height}", http.StatusBadRequest)
		return
	}
	
	width, err := strconv.ParseFloat(dimensions[0], 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid width value: %s", dimensions[0]), http.StatusBadRequest)
		return
	}
	
	height, err := strconv.ParseFloat(dimensions[1], 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid height value: %s", dimensions[1]), http.StatusBadRequest)
		return
	}
	
	// Parse index
	index, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid index value: %s", parts[3]), http.StatusBadRequest)
		return
	}
	
	// Get ZPL data
	var zplData []byte
	
	if r.Method == http.MethodGet {
		// For GET, ZPL is in the URL path or query string
		if len(parts) > 4 {
			// ZPL is in the path
			zplEncoded := strings.Join(parts[4:], "/")
			decoded, err := url.QueryUnescape(zplEncoded)
			if err != nil {
				http.Error(w, "Failed to decode ZPL from URL", http.StatusBadRequest)
				return
			}
			zplData = []byte(decoded)
		} else {
			// Try to get from query string
			zplData = []byte(r.URL.RawQuery)
		}
	} else if r.Method == http.MethodPost {
		// For POST, ZPL is in the request body
		contentType := r.Header.Get("Content-Type")
		
		if strings.Contains(contentType, "multipart/form-data") {
			// Parse multipart form
			err := r.ParseMultipartForm(32 << 20) // 32 MB max
			if err != nil {
				http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
				return
			}
			
			file, _, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "No 'file' field in multipart form", http.StatusBadRequest)
				return
			}
			defer file.Close()
			
			zplData, err = io.ReadAll(file)
			if err != nil {
				http.Error(w, "Failed to read file from form", http.StatusInternalServerError)
				return
			}
		} else {
			// application/x-www-form-urlencoded or raw body
			zplData, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusInternalServerError)
				return
			}
		}
	} else {
		http.Error(w, "Method not allowed. Use GET or POST", http.StatusMethodNotAllowed)
		return
	}
	
	if len(zplData) == 0 {
		http.Error(w, "No ZPL data provided", http.StatusBadRequest)
		return
	}
	
	// Parse ZPL
	parser := zebrash.NewParser()
	labels, err := parser.Parse(zplData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse ZPL: %v", err), http.StatusBadRequest)
		return
	}
	
	if len(labels) == 0 {
		http.Error(w, "No labels found in ZPL data", http.StatusBadRequest)
		return
	}
	
	// Check if index is valid
	if index < 0 || index >= len(labels) {
		http.Error(w, fmt.Sprintf("Invalid index %d. Found %d labels", index, len(labels)), http.StatusBadRequest)
		return
	}
	
	// Convert dimensions from inches to mm (Labelary uses inches, zebrash uses mm)
	widthMm := width * 25.4
	heightMm := height * 25.4
	
	// Render label
	drawer := zebrash.NewDrawer()
	var buf bytes.Buffer
	
	err = drawer.DrawLabelAsPng(labels[index], &buf, drawers.DrawerOptions{
		LabelWidthMm:  widthMm,
		LabelHeightMm: heightMm,
		Dpmm:          dpmm,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to render label: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Handle rotation if X-Rotation header is present
	rotation := r.Header.Get("X-Rotation")
	if rotation != "" && rotation != "0" {
		rotDegrees, err := strconv.Atoi(rotation)
		if err != nil || (rotDegrees != 90 && rotDegrees != 180 && rotDegrees != 270) {
			http.Error(w, "Invalid X-Rotation value. Must be 0, 90, 180, or 270", http.StatusBadRequest)
			return
		}
		
		// Decode the image
		img, err := png.Decode(&buf)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode image for rotation: %v", err), http.StatusInternalServerError)
			return
		}
		
		// Rotate the image
		rotatedImg := rotateImage(img, rotDegrees)
		
		// Re-encode to buffer
		buf.Reset()
		err = png.Encode(&buf, rotatedImg)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to encode rotated image: %v", err), http.StatusInternalServerError)
			return
		}
	}
	
	// Set response headers
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Total-Count", strconv.Itoa(len(labels)))
	w.WriteHeader(http.StatusOK)
	
	// Write image data
	_, err = w.Write(buf.Bytes())
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// rotateImage rotates an image by the specified degrees clockwise
func rotateImage(img image.Image, degrees int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	var rotated *image.RGBA
	
	switch degrees {
	case 90:
		// Rotate 90 degrees clockwise
		rotated = image.NewRGBA(image.Rect(0, 0, height, width))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				rotated.Set(height-1-y, x, img.At(x, y))
			}
		}
	case 180:
		// Rotate 180 degrees
		rotated = image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				rotated.Set(width-1-x, height-1-y, img.At(x, y))
			}
		}
	case 270:
		// Rotate 270 degrees clockwise (90 counter-clockwise)
		rotated = image.NewRGBA(image.Rect(0, 0, height, width))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				rotated.Set(y, width-1-x, img.At(x, y))
			}
		}
	default:
		// No rotation or 0 degrees
		rotated = image.NewRGBA(bounds)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				rotated.Set(x, y, img.At(x, y))
			}
		}
	}
	
	return rotated
}
