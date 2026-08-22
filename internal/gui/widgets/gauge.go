package widgets

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Gauge displays a capacity/in-flight meter with colored status levels.
type Gauge struct {
	Label   string
	Current int64
	Max     int64
	Height  unit.Dp
}

// Layout renders the gauge bar with text labels.
func (g Gauge) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if g.Height == 0 {
		g.Height = unit.Dp(8)
	}

	ratio := float32(0.0)
	if g.Max > 0 {
		ratio = float32(g.Current) / float32(g.Max)
		if ratio > 1.0 {
			ratio = 1.0
		}
		if ratio < 0.0 {
			ratio = 0.0
		}
	}

	// Dynamic color depending on utilization
	barColor := color.NRGBA{R: 31, G: 111, B: 235, A: 255} // Blue
	if ratio >= 0.90 {
		barColor = color.NRGBA{R: 218, G: 54, B: 51, A: 255} // Red
	} else if ratio >= 0.75 {
		barColor = color.NRGBA{R: 210, G: 153, B: 34, A: 255} // Amber
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
				Spacing:   layout.SpaceBetween,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(12), g.Label)
					lbl.Color = color.NRGBA{R: 139, G: 148, B: 158, A: 255}
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					valText := fmt.Sprintf("%d / %d (%d%%)", g.Current, g.Max, int(ratio*100))
					lbl := material.Label(th, unit.Sp(12), valText)
					lbl.Color = color.NRGBA{R: 240, G: 246, B: 252, A: 255}
					return lbl.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			totalWidth := gtx.Constraints.Max.X
			h := gtx.Dp(g.Height)
			r := h / 2

			// Background track
			trackShape := clip.RRect{
				Rect: image.Rect(0, 0, totalWidth, h),
				NW:   r, NE: r, SE: r, SW: r,
			}
			paint.FillShape(gtx.Ops, color.NRGBA{R: 48, G: 54, B: 61, A: 255}, trackShape.Op(gtx.Ops))

			// Active filled progress
			filledWidth := int(float32(totalWidth) * ratio)
			if filledWidth > 0 {
				fillShape := clip.RRect{
					Rect: image.Rect(0, 0, filledWidth, h),
					NW:   r, NE: r, SE: r, SW: r,
				}
				paint.FillShape(gtx.Ops, barColor, fillShape.Op(gtx.Ops))
			}

			return layout.Dimensions{Size: image.Point{X: totalWidth, Y: h}}
		}),
	)
}
