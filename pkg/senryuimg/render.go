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

	"github.com/rivo/uniseg"

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
	loadedFont         *truetype.Font
	loadedFallbackFont *truetype.Font
	fontPath           = "data/fonts/kouzan.ttf"
	fallbackFontPath   string
)

// SetFontPath overrides the default font path.
func SetFontPath(path string) {
	fontPath = path
}

// SetFallbackFontPath sets the fallback font path for missing glyphs.
func SetFallbackFontPath(path string) {
	fallbackFontPath = path
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

func getFallbackFont() *truetype.Font {
	if loadedFallbackFont != nil {
		return loadedFallbackFont
	}
	if fallbackFontPath == "" {
		return nil
	}
	fontBytes, err := os.ReadFile(fallbackFontPath)
	if err != nil {
		logger.Warn("Failed to read fallback font file", "path", fallbackFontPath, "error", err)
		return nil
	}
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		logger.Warn("Failed to parse fallback font", "error", err)
		return nil
	}
	loadedFallbackFont = f
	return loadedFallbackFont
}

// hasGlyph checks if the font has a glyph for the given rune.
func hasGlyph(f *truetype.Font, ch rune) bool {
	return f.Index(ch) != 0
}

// splitGraphemes splits a string into grapheme clusters (correctly handles emoji).
// Each element is a single visual character (which may be multiple runes for emoji).
func splitGraphemes(s string) []string {
	var clusters []string
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		clusters = append(clusters, gr.Str())
	}
	return clusters
}

// countGraphemes returns the number of grapheme clusters in a string.
func countGraphemes(s string) int {
	return uniseg.GraphemeClusterCount(s)
}

// RenderSenryu generates a senryu image and returns it as webp bytes.
func RenderSenryu(opts RenderOptions) ([]byte, error) {
	f, err := getFont()
	if err != nil {
		return nil, err
	}

	// Build phrase list dynamically
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

	numCols := len(phrases)
	if numCols == 0 {
		return nil, fmt.Errorf("no phrases to render")
	}

	// Find the longest phrase (using grapheme clusters for correct emoji counting)
	maxChars := 0
	for _, p := range phrases {
		n := countGraphemes(p)
		if n > maxChars {
			maxChars = n
		}
	}

	// Font settings — auto-scale to fit max height
	const maxImgHeight = 600.0
	mainFontSize := 56.0

	// Calculate layout with current font size and check if it fits
	calcLayout := func(fontSize float64) (charH, authCharH, authFontSz, colSpc, pX, pTop, pBot, stg, poemH, authAreaH float64, w, h int) {
		authFontSz = fontSize * 0.43
		charH = fontSize * 1.2
		authCharH = authFontSz * 1.3
		colSpc = fontSize * 1.4
		pX = fontSize * 0.8
		pTop = fontSize * 0.6
		pBot = fontSize * 0.5
		stg = charH * 0.5

		if numCols == 1 {
			colSpc = 0
			stg = 0
		}

		authAreaH = 0
		if opts.AuthorName != "" {
			authNameChars := countGraphemes(opts.AuthorName)
			authAreaH = float64(authNameChars)*authCharH + authFontSz*0.5
			if opts.AvatarURL != "" {
				authAreaH += fontSize * 0.85
			}
		}

		poemH = float64(maxChars)*charH + float64(numCols-1)*stg

		if numCols == 1 {
			// Single column: author placed as a 2nd column to the left
			// Width needs room for poem + author column
			authorColW := authFontSz * 1.5
			if opts.AuthorName != "" {
				w = int(pX*2 + fontSize + authorColW)
			} else {
				w = int(pX*2 + fontSize)
			}
			// Height is the taller of poem or author area
			h = int(pTop + math.Max(poemH, authAreaH) + pBot)
		} else {
			w = int(pX*2 + float64(numCols-1)*colSpc + fontSize)
			h = int(pTop + poemH + pBot + authAreaH)
		}
		return
	}

	// Scale down font if image would be too tall
	_, _, _, _, _, _, _, _, _, _, _, testH := calcLayout(mainFontSize)
	if float64(testH) > maxImgHeight {
		mainFontSize = mainFontSize * maxImgHeight / float64(testH)
		if mainFontSize < 20 {
			mainFontSize = 20
		}
	}

	_, _, authorFontSize, colSpacing, padX, padTop, _, stagger, poemHeight, _, imgWidth, imgHeight := calcLayout(mainFontSize)

	// Minimum width/height
	if imgWidth < 200 {
		imgWidth = 200
	}
	if imgHeight < 200 {
		imgHeight = 200
	}

	authorChars := []rune(opts.AuthorName)

	// Create base image
	var baseImg *image.RGBA
	if opts.Background != nil {
		bg, _, decErr := image.Decode(bytes.NewReader(opts.Background))
		if decErr != nil {
			logger.Warn("Failed to decode background image, using white", "error", decErr)
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

	mainFace := truetype.NewFace(f, &truetype.Options{
		Size:    mainFontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	defer mainFace.Close()

	authorFace := truetype.NewFace(f, &truetype.Options{
		Size:    authorFontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	defer authorFace.Close()

	textColor := color.RGBA{R: 30, G: 30, B: 30, A: 255}

	// Draw each phrase as a vertical column (right to left, staggered 575 Online style)
	startX := float64(imgWidth) - padX - mainFontSize/2
	cHeight := mainFontSize * 1.2

	for col, phrase := range phrases {
		x := startX - float64(col)*colSpacing
		graphemes := splitGraphemes(phrase)

		// 575 Online style: each column starts progressively lower
		yOffset := padTop + float64(col)*stagger

		for i, cluster := range graphemes {
			y := yOffset + float64(i)*cHeight + mainFontSize
			runes := []rune(cluster)
			if len(runes) == 1 {
				drawChar(dc, mainFace, textColor, x, y, runes[0])
			} else {
				// Multi-rune grapheme cluster (e.g., emoji) — draw as string
				drawString(dc, mainFace, textColor, x, y, cluster)
			}
		}
	}

	// Draw author name (vertical)
	if opts.AuthorName != "" {
		aCharH := authorFontSize * 1.3
		var authorX, authorStartY float64

		if numCols == 1 {
			// Single column: author placed as a left column, starting from top
			authorX = startX - mainFontSize*0.8
			authorStartY = padTop + poemHeight*0.3
		} else {
			// Multi-column: author placed left of last column, below poem
			authorX = startX - float64(numCols-1)*colSpacing - colSpacing*0.5
			authorStartY = padTop + poemHeight + mainFontSize*0.3
		}

		for i, ch := range authorChars {
			y := authorStartY + float64(i)*aCharH + authorFontSize
			drawChar(dc, authorFace, textColor, authorX, y, ch)
		}

		// Draw hanko below author name
		if opts.AvatarURL != "" {
			hankoY := authorStartY + float64(len(authorChars))*aCharH + 10
			hankoSize := mainFontSize * 0.85
			hankoX := authorX - hankoSize/2 + authorFontSize/2

			if hankoErr := drawHanko(dc, opts.AvatarURL, hankoX, hankoY, hankoSize); hankoErr != nil {
				logger.Warn("Failed to draw hanko", "error", hankoErr)
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

// verticalCharMap maps characters to their vertical writing equivalents.
var verticalCharMap = map[rune]rune{
	'「': '﹁',
	'」': '﹂',
	'『': '﹃',
	'』': '﹄',
	'（': '︵',
	'）': '︶',
	'(': '︵',
	')': '︶',
	'【': '︻',
	'】': '︼',
	'〔': '︹',
	'〕': '︺',
	'｛': '︷',
	'｝': '︸',
	'〈': '︿',
	'〉': '﹀',
	'《': '︽',
	'》': '︾',
	'。': '︒',
	'、': '︑',
}

// isLongVowelMark returns true for characters that should be drawn as a vertical line.
func isLongVowelMark(ch rune) bool {
	return ch == 'ー' || ch == '～' || ch == '〜' || ch == '—' || ch == '―'
}

// drawChar draws a single character at the given position.
// Handles vertical writing transformations for brackets, punctuation, and long vowel marks.
// Falls back to the fallback font if the main font doesn't have the glyph.
func drawChar(dc *gg.Context, face font.Face, col color.Color, x, y float64, ch rune) {
	// Handle long vowel mark (ー) — draw as vertical line
	if isLongVowelMark(ch) {
		r, g, b, _ := col.RGBA()
		dc.SetColor(color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255})
		metrics := face.Metrics()
		lineH := float64(metrics.Ascent.Round()) * 0.7
		lineW := 2.5
		dc.DrawRoundedRectangle(x-lineW/2, y-lineH, lineW, lineH, lineW/2)
		dc.Fill()
		return
	}

	// Map to vertical writing form if available
	if vert, ok := verticalCharMap[ch]; ok {
		ch = vert
	}

	// Check if main font has the glyph; if not, try fallback
	useFace := face
	if loadedFont != nil && !hasGlyph(loadedFont, ch) {
		if fb := getFallbackFont(); fb != nil {
			// Create a fallback face with the same metrics as the main face
			m := face.Metrics()
			fontSize := float64(m.Ascent.Round()+m.Descent.Round()) * 0.85
			fbFace := truetype.NewFace(fb, &truetype.Options{
				Size:    fontSize,
				DPI:     72,
				Hinting: font.HintingFull,
			})
			useFace = fbFace
			defer fbFace.Close()
		}
	}

	d := &font.Drawer{
		Dst:  dc.Image().(*image.RGBA),
		Src:  image.NewUniform(col),
		Face: useFace,
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

// drawString draws a multi-rune string (e.g., emoji grapheme cluster) centered at position.
func drawString(dc *gg.Context, face font.Face, col color.Color, x, y float64, s string) {
	useFace := face
	// Try fallback font for emoji / missing glyphs
	runes := []rune(s)
	if loadedFont != nil && len(runes) > 0 && !hasGlyph(loadedFont, runes[0]) {
		if fb := getFallbackFont(); fb != nil {
			m := face.Metrics()
			fontSize := float64(m.Ascent.Round()+m.Descent.Round()) * 0.85
			fbFace := truetype.NewFace(fb, &truetype.Options{
				Size:    fontSize,
				DPI:     72,
				Hinting: font.HintingFull,
			})
			useFace = fbFace
			defer fbFace.Close()
		}
	}

	d := &font.Drawer{
		Dst:  dc.Image().(*image.RGBA),
		Src:  image.NewUniform(col),
		Face: useFace,
	}
	advance := d.MeasureString(s)
	advPx := float64(advance >> 6)
	d.Dot = fixed.Point26_6{
		X: fixed.I(int(x - advPx/2)),
		Y: fixed.I(int(y)),
	}
	d.DrawString(s)
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
