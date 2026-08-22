package widgets

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Logo renders a minimalist vector geometric emblem for DAEGSA.
// The emblem symbolizes arrival-rate traffic waves forming a modern stylized "D".
type Logo struct {
	Size unit.Dp
}

// Layout renders the vector emblem.
func (l Logo) Layout(gtx layout.Context) layout.Dimensions {
	sz := l.Size
	if sz == 0 {
		sz = unit.Dp(32)
	}

	sizePx := gtx.Dp(sz)
	if sizePx < 16 {
		sizePx = 16
	}

	s := float32(sizePx)

	// Container rounded badge with subtle border
	r := s * 0.22
	badgeRect := image.Rect(0, 0, sizePx, sizePx)
	badgeShape := clip.RRect{
		Rect: badgeRect,
		NW:   int(r), NE: int(r), SE: int(r), SW: int(r),
	}

	// Deep gradient-like background surface
	paint.FillShape(gtx.Ops, color.NRGBA{R: 22, G: 27, B: 34, A: 255}, badgeShape.Op(gtx.Ops))

	// Outer subtle glowing accent border
	paint.FillShape(gtx.Ops, color.NRGBA{R: 56, G: 139, B: 253, A: 120}, clip.Stroke{
		Path:  badgeShape.Path(gtx.Ops),
		Width: float32(gtx.Dp(unit.Dp(1))),
	}.Op())

	// Geometric "D" + Traffic Surge Wave Bars
	// Bar 1 (Left Pillar): Base of the D
	b1Width := s * 0.16
	b1Height := s * 0.60
	b1X := s * 0.22
	b1Y := s * 0.20
	b1R := b1Width * 0.35

	bar1Shape := clip.RRect{
		Rect: image.Rect(int(b1X), int(b1Y), int(b1X+b1Width), int(b1Y+b1Height)),
		NW:   int(b1R), NE: int(b1R), SE: int(b1R), SW: int(b1R),
	}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 88, G: 166, B: 255, A: 255}, bar1Shape.Op(gtx.Ops)) // Bright Cyan

	// Bar 2 (Middle Surge Bar - 45% height)
	b2Width := s * 0.14
	b2Height := s * 0.38
	b2X := s * 0.44
	b2Y := s * 0.42
	b2R := b2Width * 0.35

	bar2Shape := clip.RRect{
		Rect: image.Rect(int(b2X), int(b2Y), int(b2X+b2Width), int(b2Y+b2Height)),
		NW:   int(b2R), NE: int(b2R), SE: int(b2R), SW: int(b2R),
	}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 163, G: 113, B: 247, A: 255}, bar2Shape.Op(gtx.Ops)) // Purple Accent

	// Bar 3 (Right Crest Bar - 65% height, completing the "D" contour)
	b3Width := s * 0.14
	b3Height := s * 0.50
	b3X := s * 0.64
	b3Y := s * 0.30
	b3R := b3Width * 0.35

	bar3Shape := clip.RRect{
		Rect: image.Rect(int(b3X), int(b3Y), int(b3X+b3Width), int(b3Y+b3Height)),
		NW:   int(b3R), NE: int(b3R), SE: int(b3R), SW: int(b3R),
	}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 210, G: 153, B: 34, A: 255}, bar3Shape.Op(gtx.Ops)) // Amber Spark

	// Upper wave connecting arc
	var arcPath clip.Path
	arcPath.Begin(gtx.Ops)
	arcPath.MoveTo(f32.Pt(b1X+b1Width*0.5, b1Y+b1R))
	arcPath.QuadTo(f32.Pt(s*0.55, s*0.14), f32.Pt(b3X+b3Width*0.5, b3Y+b3R))
	paint.FillShape(gtx.Ops, color.NRGBA{R: 88, G: 166, B: 255, A: 200}, clip.Stroke{
		Path:  arcPath.End(),
		Width: float32(gtx.Dp(unit.Dp(1.5))),
	}.Op())

	return layout.Dimensions{Size: image.Point{X: sizePx, Y: sizePx}}
}

// BrandHeader renders the logo emblem alongside the minimalist DAEGSA brand typography.
type BrandHeader struct {
	Size unit.Dp
}

// Layout renders the brand header.
func (b BrandHeader) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return Logo{Size: b.Size}.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.H6(th, "DAEGSA")
					lbl.Color = color.NRGBA{R: 240, G: 246, B: 252, A: 255}
					lbl.TextSize = unit.Sp(16)
					return lbl.Layout(gtx)
				}),
				// layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// 	lbl := material.Label(th, unit.Sp(10), "")
				// 	lbl.Color = color.NRGBA{R: 88, G: 166, B: 255, A: 200}
				// 	return lbl.Layout(gtx)
				// }),
			)
		}),
	)
}
