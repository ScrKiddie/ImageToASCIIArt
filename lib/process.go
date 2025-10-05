package lib

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strconv"
	"strings"
	"sync"
	"syscall/js"

	"github.com/ajstarks/svgo"
	"github.com/disintegration/imaging"
	"github.com/leaanthony/go-ansi-parser"
	"github.com/qeesung/image2ascii/convert"
)

const (
	MaxImageSize      = 50 * 1024 * 1024
	MaxOutputSize     = 10 * 1024 * 1024
	MaxASCIIChars     = 5000000
	MaxASCIIDimension = 500
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type Options struct {
	TargetWidth           int
	Brightness            float64
	Contrast              float64
	Sharpen               float64
	BackgroundColor       string
	TransparencyColor     string
	TransparencyThreshold float64
}

func ProcessImageToSVG(imageData []byte, opts Options) (string, error) {
	if err := validateInput(imageData, opts); err != nil {
		return "", err
	}
	opts.setDefaults()

	img, format, err := decodeImage(imageData)
	if err != nil {
		return "", err
	}
	fmt.Printf("Image decoded successfully. Format: %s\n", format)

	processedImg := processImage(img, opts)

	asciiString, err := convertToASCII(processedImg, opts.TargetWidth)
	if err != nil {
		return "", err
	}

	styledText, err := parseANSI(asciiString)
	if err != nil {
		return "", err
	}

	svgString, err := renderToSVG(styledText, opts.BackgroundColor)
	if err != nil {
		return "", err
	}

	if len(svgString) > MaxOutputSize {
		return "", fmt.Errorf("output SVG is too large: %d bytes (max: %d)", len(svgString), MaxOutputSize)
	}

	return svgString, nil
}

func validateInput(imageData []byte, opts Options) error {
	if len(imageData) == 0 {
		return fmt.Errorf("image data is empty")
	}
	if len(imageData) > MaxImageSize {
		return fmt.Errorf("image data is too large: %d bytes (max: %d)", len(imageData), MaxImageSize)
	}
	if opts.TargetWidth <= 0 {
		return fmt.Errorf("target width must be positive")
	}
	return nil
}

func (o *Options) setDefaults() {
	if o.BackgroundColor == "" {
		o.BackgroundColor = "#000000"
	}
	if o.TransparencyColor == "" {
		o.TransparencyColor = "#FFFFFF"
	}
	o.TransparencyThreshold = math.Max(0.0, math.Min(1.0, o.TransparencyThreshold))
}

func decodeImage(imageData []byte) (image.Image, string, error) {
	buffer := bytes.NewReader(imageData)
	img, format, err := image.Decode(buffer)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}
	if img == nil {
		return nil, "", fmt.Errorf("decoded image is nil")
	}

	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, "", fmt.Errorf("invalid image dimensions: %dx%d", bounds.Dx(), bounds.Dy())
	}

	return img, format, nil
}

func processImage(img image.Image, opts Options) image.Image {
	bounds := img.Bounds()
	originalWidth, originalHeight := bounds.Dx(), bounds.Dy()
	fmt.Printf("Original image dimensions: %dx%d\n", originalWidth, originalHeight)

	const maxProcessDimension = 1024
	if originalWidth > maxProcessDimension || originalHeight > maxProcessDimension {
		scale := float64(maxProcessDimension) / float64(max(originalWidth, originalHeight))
		newWidth := int(float64(originalWidth) * scale)
		newHeight := int(float64(originalHeight) * scale)
		if newWidth < 1 {
			newWidth = 1
		}
		if newHeight < 1 {
			newHeight = 1
		}
		fmt.Printf("Resizing to: %dx%d (scale: %.2f)\n", newWidth, newHeight, scale)
		img = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
	}

	if opts.Brightness != 0 {
		img = imaging.AdjustBrightness(img, opts.Brightness)
	}
	if opts.Contrast != 0 {
		img = imaging.AdjustContrast(img, opts.Contrast)
	}
	if opts.Sharpen != 0 {
		img = imaging.Sharpen(img, opts.Sharpen)
	}

	return handleTransparency(img, opts.TransparencyColor, opts.TransparencyThreshold)
}

func convertToASCII(img image.Image, targetWidth int) (string, error) {
	options := convert.DefaultOptions
	options.FixedWidth = targetWidth
	options.Colored = true
	options.StretchedScreen = false

	bounds := img.Bounds()
	aspectRatio := float64(bounds.Dy()) / float64(bounds.Dx())
	options.FixedHeight = int(float64(targetWidth) * aspectRatio)
	if options.FixedHeight < 1 {
		options.FixedHeight = 1
	}

	if options.FixedWidth > MaxASCIIDimension {
		scale := float64(MaxASCIIDimension) / float64(options.FixedWidth)
		options.FixedWidth = MaxASCIIDimension
		options.FixedHeight = int(float64(options.FixedHeight) * scale)
	}
	if options.FixedHeight > MaxASCIIDimension {
		scale := float64(MaxASCIIDimension) / float64(options.FixedHeight)
		options.FixedHeight = MaxASCIIDimension
		options.FixedWidth = int(float64(options.FixedWidth) * scale)
	}

	fmt.Printf("Original: %dx%d, ASCII: %dx%d, Ratio: %.2f\n",
		bounds.Dx(), bounds.Dy(), options.FixedWidth, options.FixedHeight, aspectRatio)

	converter := convert.NewImageConverter()
	asciiString := converter.Image2ASCIIString(img, &options)
	if asciiString == "" {
		return "", fmt.Errorf("failed to convert image to ASCII")
	}

	if len(asciiString) > MaxASCIIChars {
		return "", fmt.Errorf("ASCII output is too large: %s characters (max: %s)",
			formatNumber(len(asciiString)), formatNumber(MaxASCIIChars))
	}
	if len(asciiString) > 3_000_000 {
		js.Global().Get("console").Call("warn",
			fmt.Sprintf("Very large ASCII output: %s characters. Processing may take time.", formatNumber(len(asciiString))))
	} else if len(asciiString) > 1_000_000 {
		js.Global().Get("console").Call("log",
			fmt.Sprintf("Large ASCII output: %s characters.", formatNumber(len(asciiString))))
	}

	return asciiString, nil
}

func parseANSI(asciiString string) ([]*ansi.StyledText, error) {
	if asciiString == "" {
		return nil, fmt.Errorf("ASCII string is empty")
	}
	styledText, err := ansi.Parse(asciiString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ANSI string: %w", err)
	}
	if styledText == nil {
		return nil, fmt.Errorf("styled text is nil after parsing")
	}

	const maxStyledElements = 100_000
	if len(styledText) > maxStyledElements {
		return nil, fmt.Errorf("too many styled text elements: %d (max: %d)", len(styledText), maxStyledElements)
	}
	if len(styledText) > 30_000 {
		js.Global().Get("console").Call("warn",
			fmt.Sprintf("Large number of styled text elements: %d. Processing may be slower.", len(styledText)))
	}

	return styledText, nil
}

const (
	lineHeight    = 16
	charWidth     = 16
	fontSize      = 16
	paddingTop    = -2
	paddingBottom = 2
	paddingLeft   = 1
	paddingRight  = -6
)

func renderToSVG(styledText []*ansi.StyledText, backgroundColor string) (string, error) {
	if styledText == nil {
		return "", fmt.Errorf("styledText is nil")
	}

	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer bufferPool.Put(buffer)

	canvas := svg.New(buffer)
	lines := splitStyledTextByLine(styledText)
	svgWidth, svgHeight := calculateSVGDimensions(lines)

	canvas.Start(svgWidth, svgHeight)
	canvas.Rect(0, 0, svgWidth, svgHeight, fmt.Sprintf("fill:%s", backgroundColor))

	yPos := paddingTop
	for _, line := range lines {
		renderLine(canvas, line, yPos, paddingLeft)
		yPos += lineHeight
	}

	canvas.End()
	return buffer.String(), nil
}

func calculateSVGDimensions(lines [][]*ansi.StyledText) (width, height int) {
	maxLineLength := 0
	for _, line := range lines {
		currentLineLength := 0
		for _, styledChar := range line {
			currentLineLength += len(styledChar.Label)
		}
		if currentLineLength > maxLineLength {
			maxLineLength = currentLineLength
		}
	}

	width = (maxLineLength * charWidth) + paddingLeft + paddingRight
	height = (len(lines) * lineHeight) + paddingTop + paddingBottom

	if width <= 0 {
		width = charWidth + paddingLeft + paddingRight
	}
	if height <= 0 {
		height = lineHeight + paddingTop + paddingBottom
	}

	fmt.Printf("SVG dimensions: %dx%d (based on %d lines, max length: %d)\n", width, height, len(lines), maxLineLength)
	return width, height
}

func renderLine(canvas *svg.SVG, line []*ansi.StyledText, yPos, startX int) {
	currentX := startX
	for _, styledChar := range line {
		if styledChar.Label == "" {
			continue
		}

		if styledChar.Label == " " {
			currentX += charWidth
			continue
		}

		textColor := "#FFFFFF"
		if styledChar.FgCol != nil && styledChar.FgCol.Hex != "" {
			textColor = styledChar.FgCol.Hex
		}

		style := fmt.Sprintf("fill:%s; font-family:monospace; font-size:%dpx; dominant-baseline:text-before-edge", textColor, fontSize)
		canvas.Text(currentX, yPos, styledChar.Label, style)
		currentX += len(styledChar.Label) * charWidth
	}
}

func handleTransparency(img image.Image, transparencyColorStr string, threshold float64) image.Image {
	tColor := parseHexColor(transparencyColorStr)
	if tColor == nil {
		tColor = color.White
	}

	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	alphaThreshold := uint32(math.Floor(threshold * 65535))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			originalColor := img.At(x, y)
			r, g, b, a := originalColor.RGBA()

			if a < alphaThreshold {
				result.Set(x, y, tColor)
			} else if a < 0xFFFF {
				alphaFactor := float64(a) / 65535.0
				tr, tg, tb, _ := tColor.RGBA()

				blendR := (float64(r)/65535.0)*alphaFactor + (float64(tr)/65535.0)*(1-alphaFactor)
				blendG := (float64(g)/65535.0)*alphaFactor + (float64(tg)/65535.0)*(1-alphaFactor)
				blendB := (float64(b)/65535.0)*alphaFactor + (float64(tb)/65535.0)*(1-alphaFactor)

				result.Set(x, y, color.RGBA{
					R: uint8(blendR * 255),
					G: uint8(blendG * 255),
					B: uint8(blendB * 255),
					A: 255,
				})
			} else {
				result.Set(x, y, originalColor)
			}
		}
	}
	return result
}

func parseHexColor(hex string) color.Color {
	hex = strings.TrimPrefix(hex, "#")

	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return nil
	}

	r, errR := strconv.ParseInt(hex[0:2], 16, 64)
	g, errG := strconv.ParseInt(hex[2:4], 16, 64)
	b, errB := strconv.ParseInt(hex[4:6], 16, 64)

	if errR != nil || errG != nil || errB != nil {
		return nil
	}

	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xFF}
}

func splitStyledTextByLine(styledText []*ansi.StyledText) [][]*ansi.StyledText {
	var lines [][]*ansi.StyledText
	var currentLine []*ansi.StyledText

	for _, block := range styledText {
		if block == nil {
			continue
		}
		parts := strings.Split(block.Label, "\n")
		for i, part := range parts {
			if part != "" {
				newBlock := &ansi.StyledText{
					Label: part,
					FgCol: block.FgCol,
					BgCol: block.BgCol,
					Style: block.Style,
				}
				currentLine = append(currentLine, newBlock)
			}
			if i < len(parts)-1 {
				lines = append(lines, currentLine)
				currentLine = nil
			}
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}
	return lines
}

func formatNumber(n int) string {
	in := strconv.Itoa(n)
	if n < 0 {
		in = in[1:]
	}
	numOfCommas := (len(in) - 1) / 3
	out := make([]byte, len(in)+numOfCommas)
	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			if n < 0 {
				return "-" + string(out)
			}
			return string(out)
		}
		if k++; k == 3 {
			j--
			k = 0
			out[j] = ','
		}
	}
}
