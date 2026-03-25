package senryuimg

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	// webp decoder is registered by github.com/chai2010/webp import
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chai2010/webp"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
)

// RenderOptions holds options for rendering a senryu image.
type RenderOptions struct {
	Kamigo     string // upper phrase (5 mora)
	Nakasichi  string // middle phrase (7 mora)
	Simogo     string // lower phrase (5 mora)
	Shiku      string // 4th phrase (7 mora, tanka only)
	Goku       string // 5th phrase (7 mora, tanka only)
	AuthorName string // display name
	AvatarURL  string // Discord avatar URL
	Background []byte // custom background image (nil = white)
}

var (
	loadedFont *truetype.Font
	fontPath   = "data/fonts/kouzan.ttf"
)

// SetFontPath overrides the default font path.
func SetFontPath(path string) {
	fontPath = path
}

func getFont() (*truetype.Font, error) {
	if loadedFont != nil {
		return loadedFont, nil
	}
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read font file %s: %w", fontPath, err)
	}
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %w", err)
	}
	loadedFont = f
	return loadedFont, nil
}

// RenderSenryu generates a senryu image and returns it as webp bytes.
func RenderSenryu(opts RenderOptions) ([]byte, error) {
	f, err := getFont()
	if err != nil {
		return nil, err
	}

	// Image dimensions
	const (
		imgWidth  = 800
		imgHeight = 1200
		padding   = 80
	)

	// Create base image
	var baseImg *image.RGBA
	if opts.Background != nil {
		bg, _, err := image.Decode(bytes.NewReader(opts.Background))
		if err != nil {
			logger.Warn("Failed to decode background image, using white", "error", err)
			baseImg = image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
			draw.Draw(baseImg, baseImg.Bounds(), image.White, image.Point{}, draw.Src)
		} else {
			baseImg = resizeToFit(bg, imgWidth, imgHeight)
		}
	} else {
		baseImg = image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
		draw.Draw(baseImg, baseImg.Bounds(), image.White, image.Point{}, draw.Src)
	}

	dc := gg.NewContextForRGBA(baseImg)

	// Main text settings
	mainFontSize := 72.0
	mainFace := truetype.NewFace(f, &truetype.Options{
		Size:    mainFontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	defer mainFace.Close()

	// Author text settings
	authorFontSize := 32.0
	authorFace := truetype.NewFace(f, &truetype.Options{
		Size:    authorFontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	defer authorFace.Close()

	// Text color
	textColor := color.RGBA{R: 30, G: 30, B: 30, A: 255}

	// Build phrase list dynamically (supports senryu, tanka, and single-line jiyuritsu)
	var phrases []string
	if opts.Kamigo != "" {
		phrases = append(phrases, opts.Kamigo)
	}
	if opts.Nakasichi != "" {
		phrases = append(phrases, opts.Nakasichi)
	}
	if opts.Simogo != "" {
		phrases = append(phrases, opts.Simogo)
	}
	if opts.Shiku != "" {
		phrases = append(phrases, opts.Shiku)
	}
	if opts.Goku != "" {
		phrases = append(phrases, opts.Goku)
	}

	// Adjust column spacing based on number of phrases
	numCols := len(phrases)
	colSpacing := 120.0
	if numCols >= 5 {
		colSpacing = 100.0 // tighter for tanka
	} else if numCols == 1 {
		colSpacing = 0 // single column for jiyuritsu
	}

	// Calculate column positions (right to left)
	// Vertical text: each character is drawn top-to-bottom, columns flow right-to-left
	startX := float64(imgWidth) - padding - mainFontSize/2
	startY := float64(padding) + 40

	// Find the longest phrase for vertical centering
	maxChars := 0
	for _, p := range phrases {
		n := utf8.RuneCountInString(p)
		if n > maxChars {
			maxChars = n
		}
	}

	// Draw each phrase as a vertical column (right to left)
	for col, phrase := range phrases {
		x := startX - float64(col)*colSpacing
		chars := []rune(phrase)

		// Vertical offset: center shorter columns relative to longest
		charHeight := mainFontSize * 1.15
		totalHeight := float64(len(chars)) * charHeight
		maxHeight := float64(maxChars) * charHeight
		yOffset := startY + (maxHeight-totalHeight)/2

		for i, ch := range chars {
			y := yOffset + float64(i)*charHeight + mainFontSize
			drawChar(dc, mainFace, textColor, x, y, ch)
		}
	}

	// Draw author name (smaller, vertical, to the left of the last phrase column)
	if opts.AuthorName != "" {
		authorX := startX - float64(numCols)*colSpacing + 20
		authorChars := []rune(opts.AuthorName)
		authorCharHeight := authorFontSize * 1.15

		// Position author name at bottom area
		authorStartY := float64(imgHeight) - padding - float64(len(authorChars))*authorCharHeight - 100

		for i, ch := range authorChars {
			y := authorStartY + float64(i)*authorCharHeight + authorFontSize
			drawChar(dc, authorFace, textColor, authorX, y, ch)
		}

		// Draw hanko (stamp) below author name
		if opts.AvatarURL != "" {
			hankoY := authorStartY + float64(len(authorChars))*authorCharHeight + 20
			hankoSize := 60.0
			hankoX := authorX - hankoSize/2 + authorFontSize/2

			if err := drawHanko(dc, opts.AvatarURL, hankoX, hankoY, hankoSize); err != nil {
				logger.Warn("Failed to draw hanko", "error", err)
			}
		}
	}

	// Encode to webp
	var buf bytes.Buffer
	if err := webp.Encode(&buf, dc.Image(), &webp.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("failed to encode webp: %w", err)
	}

	return buf.Bytes(), nil
}

// drawChar draws a single character at the given position.
func drawChar(dc *gg.Context, face font.Face, col color.Color, x, y float64, ch rune) {
	d := &font.Drawer{
		Dst:  dc.Image().(*image.RGBA),
		Src:  image.NewUniform(col),
		Face: face,
	}

	// Measure the character width for centering
	advance := d.MeasureString(string(ch))
	advPx := float64(advance >> 6)

	d.Dot = fixed.Point26_6{
		X: fixed.I(int(x - advPx/2)),
		Y: fixed.I(int(y)),
	}
	d.DrawString(string(ch))
}

// drawHanko draws the user's avatar as a circular red stamp.
func drawHanko(dc *gg.Context, avatarURL string, x, y, size float64) error {
	avatarImg, err := fetchImage(avatarURL)
	if err != nil {
		return fmt.Errorf("failed to fetch avatar: %w", err)
	}

	stamp := createHankoStamp(avatarImg, int(size))

	dc.DrawImageAnchored(stamp, int(x+size/2), int(y+size/2), 0.5, 0.5)
	return nil
}

// fetchImage downloads an image from a URL.
func fetchImage(url string) (image.Image, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Try webp first for Discord avatars
	if strings.Contains(url, "cdn.discordapp.com") && !strings.Contains(url, "?") {
		url += "?size=128"
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching image", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return img, nil
}

// createHankoStamp creates a circular red-tinted stamp from an avatar image.
func createHankoStamp(avatar image.Image, size int) image.Image {
	dc := gg.NewContext(size, size)

	// Draw circular clip
	dc.DrawCircle(float64(size)/2, float64(size)/2, float64(size)/2)
	dc.Clip()

	// Draw avatar scaled to fit
	dc.DrawImageAnchored(avatar, size/2, size/2, 0.5, 0.5)

	// Apply red tint overlay for stamp effect
	stampImg := dc.Image()
	result := image.NewRGBA(image.Rect(0, 0, size, size))

	bounds := stampImg.Bounds()
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size) / 2

	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			// Check if inside circle
			dx := float64(px) - cx
			dy := float64(py) - cy
			if math.Sqrt(dx*dx+dy*dy) > radius {
				continue
			}

			r, g, b, a := stampImg.At(px, py).RGBA()
			if a == 0 {
				continue
			}

			// Convert to grayscale, then apply red tint
			gray := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256.0
			// Invert and red-tint for stamp effect
			intensity := 1.0 - gray/255.0

			newR := uint8(min(255, int(180+intensity*75)))
			newG := uint8(min(255, int(30+intensity*20)))
			newB := uint8(min(255, int(30+intensity*20)))
			newA := uint8(200) // slightly transparent for stamp feel

			result.SetRGBA(px, py, color.RGBA{R: newR, G: newG, B: newB, A: newA})
		}
	}

	// Draw circular border (red ring)
	borderDC := gg.NewContextForRGBA(result)
	borderDC.SetColor(color.RGBA{R: 180, G: 30, B: 30, A: 220})
	borderDC.SetLineWidth(2)
	borderDC.DrawCircle(float64(size)/2, float64(size)/2, float64(size)/2-1)
	borderDC.Stroke()

	return result
}

// resizeToFit resizes an image to cover the given dimensions while maintaining aspect ratio,
// then centers it on a canvas of the exact target size.
func resizeToFit(src image.Image, targetW, targetH int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	// Calculate scale to cover the target area
	scaleW := float64(targetW) / float64(srcW)
	scaleH := float64(targetH) / float64(srcH)
	scale := math.Max(scaleW, scaleH)

	dc := gg.NewContext(targetW, targetH)
	dc.Scale(scale, scale)
	offsetX := float64(targetW)/scale/2 - float64(srcW)/2
	offsetY := float64(targetH)/scale/2 - float64(srcH)/2
	dc.DrawImage(src, int(offsetX), int(offsetY))

	// Convert to RGBA
	result := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.Draw(result, result.Bounds(), dc.Image(), image.Point{}, draw.Src)
	return result
}
