package drawers

import (
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/ingridhq/zebrash/drawers"
	"github.com/ingridhq/zebrash/internal/assets"
	"github.com/ingridhq/zebrash/internal/elements"
)

var (
	font0 = mustLoadFont(assets.FontHelveticaBold)
	font1 = mustLoadFont(assets.FontDejavuSansMono)
	fontB = mustLoadFont(assets.FontDejavuSansMonoBold)
)

func NewTextFieldDrawer() *ElementDrawer {
	return &ElementDrawer{
		Draw: func(gCtx *gg.Context, element any, _ drawers.DrawerOptions, state *DrawerState) error {
			text, ok := element.(*elements.TextField)
			if !ok {
				return nil
			}

			text = adjustTextField(text)

			fontSize := text.Font.GetSize()
			scaleX := text.Font.GetScaleX()
			face := truetype.NewFace(getTffFont(text.Font), &truetype.Options{Size: fontSize})
			gCtx.SetFontFace(face)

			setLineColor(gCtx, elements.LineColorBlack)

			w, h := gCtx.MeasureString(text.Text)

			// Pass single-line height to position calculator
			x, y := getTextTopLeftPos(text, w, h, state)
			state.UpdateAutomaticTextPosition(text, w, scaleX)

			ax, ay := getTextAxAy(text)

			if scaleX != 1.0 {
				gCtx.ScaleAbout(scaleX, 1, x, y)
			}

			if rotate := text.Font.Orientation.GetDegrees(); rotate != 0 {
				gCtx.RotateAbout(gg.Radians(rotate), x, y)
			}

			defer gCtx.Identity()

			if text.Block != nil {
				maxWidth := float64(text.Block.MaxWidth) / scaleX
				lineSpacing := 1 + float64(text.Block.LineSpacing)/h
				
				// Calculate how many lines we'll actually render
				lines := gCtx.WordWrap(text.Text, maxWidth)
				if text.Block.MaxLines > 0 && len(lines) > text.Block.MaxLines {
					lines = lines[:text.Block.MaxLines]
				}
				
				// Adjust starting position for rotated text
				startY := y - h
				
				// For 90-degree rotation, if we have fewer lines than maxLines,
				// shift the starting position so first line appears at "first line" position
				if text.Font.Orientation == elements.FieldOrientation90 && len(lines) < text.Block.MaxLines {
					// Calculate how many "empty" lines we need to account for
					emptyLines := text.Block.MaxLines - len(lines)
					offsetForEmptyLines := float64(emptyLines) * h * lineSpacing
					startY = y - h - offsetForEmptyLines
				}
				
				drawStringWrappedRotated(gCtx, text.Text, x, startY, ax, ay, maxWidth, lineSpacing, text.Block.Alignment, text.Block.MaxLines, text.Font.Orientation)
			} else {
				gCtx.DrawStringAnchored(text.Text, x, y, ax, ay)
			}

			return nil
		},
	}
}

func adjustTextField(text *elements.TextField) *elements.TextField {
	fontName := text.Font.Name
	res := *text

	switch fontName {
	case "B":
		// Bold font, text in all uppercase
		res.Text = strings.ToUpper(res.Text)
	}

	return &res
}

func getTffFont(font elements.FontInfo) *truetype.Font {
	switch font.Name {
	case "0":
		return font0
	case "B":
		return fontB
	default:
		return font1
	}
}

func getTextTopLeftPos(text *elements.TextField, w, h float64, state *DrawerState) (float64, float64) {
	x, y := state.GetTextPosition(text)

	lines := 1.0
	spacing := 0.0

	if text.Block != nil {
		lines = float64(max(text.Block.MaxLines, 1))
		spacing = float64(text.Block.LineSpacing)
		w = float64(text.Block.MaxWidth)
	}

	if !text.Position.CalculateFromBottom {
		switch text.Font.Orientation {
		case elements.FieldOrientation90:
			return x + h/4, y
		case elements.FieldOrientation180:
			return x + w, y + h/4
		case elements.FieldOrientation270:
			return x + 3*h/4, y + w
		default:
			return x, y + 3*h/4
		}
	}

	// Don't apply offset for Field Blocks - drawStringWrapped handles line positioning
	if text.Block != nil {
		switch text.Font.Orientation {
		case elements.FieldOrientation90:
			return x, y
		case elements.FieldOrientation180:
			return x, y
		case elements.FieldOrientation270:
			return x, y
		default:
			return x, y
		}
	}

	offset := (lines - 1) * (h + spacing)

	switch text.Font.Orientation {
	case elements.FieldOrientation90:
		return x + offset, y
	case elements.FieldOrientation180:
		return x, y + offset
	case elements.FieldOrientation270:
		return x - offset, y
	default:
		return x, y - offset
	}
}

func getTextAxAy(text *elements.TextField) (float64, float64) {
	ax := 0.0
	ay := 0.0

	if text.Alignment == elements.FieldAlignmentRight {
		ax = 1
	}

	return ax, ay
}

func mustLoadFont(fontData []byte) *truetype.Font {
	font, err := truetype.Parse(fontData)
	if err != nil {
		panic(err.Error())
	}

	return font
}

// drawStringWrappedRotated handles multiline text with rotation awareness
func drawStringWrappedRotated(gCtx *gg.Context, s string, x, y, ax, ay, width, lineSpacing float64, align elements.TextAlignment, maxLines int, orientation elements.FieldOrientation) {
	fontHeight := gCtx.FontHeight()
	lines := gCtx.WordWrap(s, width)

	// Limit to maxLines
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	// For 90-degree rotation, reverse lines and draw direction
	reverseForRotation := orientation == elements.FieldOrientation90
	if reverseForRotation {
		// Reverse the slice of lines
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	h := float64(len(lines)) * fontHeight * lineSpacing
	h -= (lineSpacing - 1) * fontHeight

	x -= ax * width
	y -= ay * h
	switch align {
	case elements.TextAlignmentLeft, elements.TextAlignmentJustified:
		ax = 0
	case elements.TextAlignmentCenter:
		ax = 0.5
		x += width / 2
	case elements.TextAlignmentRight:
		ax = 1
		x += width
	}
	ay = 1

	lastLine := len(lines) - 1

	for i, line := range lines {
		switch {
		case align == elements.TextAlignmentJustified && i < lastLine:
			drawStringJustified(gCtx, line, x, y, ax, ay, width)
		default:
			gCtx.DrawStringAnchored(line, x, y, ax, ay)
		}

		// Reverse direction for 90-degree rotation
		if reverseForRotation {
			y -= fontHeight * lineSpacing
		} else {
			y += fontHeight * lineSpacing
		}
	}
}

// Similar to gCtx.DrawStringWrapped but supports justified alignment and line limits
func drawStringWrapped(gCtx *gg.Context, s string, x, y, ax, ay, width, lineSpacing float64, align elements.TextAlignment, maxLines int) {
	fontHeight := gCtx.FontHeight()
	lines := gCtx.WordWrap(s, width)

	// Limit to maxLines
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	h := float64(len(lines)) * fontHeight * lineSpacing
	h -= (lineSpacing - 1) * fontHeight

	x -= ax * width
	y -= ay * h
	switch align {
	case elements.TextAlignmentLeft, elements.TextAlignmentJustified:
		ax = 0
	case elements.TextAlignmentCenter:
		ax = 0.5
		x += width / 2
	case elements.TextAlignmentRight:
		ax = 1
		x += width
	}
	ay = 1

	lastLine := len(lines) - 1

	for i, line := range lines {
		switch {
		case align == elements.TextAlignmentJustified && i < lastLine:
			drawStringJustified(gCtx, line, x, y, ax, ay, width)
		default:
			gCtx.DrawStringAnchored(line, x, y, ax, ay)
		}

		y += fontHeight * lineSpacing
	}
}

func drawStringJustified(gCtx *gg.Context, line string, x, y, ax, ay, maxWidth float64) {
	words := strings.Fields(line)
	fontHeight := gCtx.FontHeight()

	totalWordWidth := 0.0
	wordsWidth := make([]float64, len(words))
	for i, word := range words {
		w, _ := gCtx.MeasureString(word)
		wordsWidth[i] = w
		totalWordWidth += w
	}

	spaceCount := len(words) - 1
	spaceWidth := 0.0
	if spaceCount > 0 {
		spaceWidth = (maxWidth - totalWordWidth) / float64(spaceCount)
		if spaceWidth < 0 {
			spaceWidth = fontHeight * 0.3
		}
	}

	cx := x
	for i, word := range words {
		gCtx.DrawStringAnchored(word, cx, y, ax, ay)
		cx += wordsWidth[i] + spaceWidth
	}
}
