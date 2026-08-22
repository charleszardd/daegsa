package widgets

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// ChartSeries represents a single data series on the chart.
type ChartSeries struct {
	Name   string
	Color  color.NRGBA
	Values []float64
}

// LineChart renders a vector line chart with multi-series data and gridlines.
type LineChart struct {
	Title     string
	Unit      string // e.g. "req/s" or "ms"
	Series    []ChartSeries
	Height    unit.Dp
	FixedMaxY float64
}

// Layout renders the line chart.
func (c LineChart) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if c.Height == 0 {
		c.Height = unit.Dp(160)
	}

	chartHeightPx := gtx.Dp(c.Height)
	totalWidthPx := gtx.Constraints.Max.X
	if totalWidthPx < 100 {
		totalWidthPx = 100
	}

	// Calculate Max Y across all series
	maxY := c.FixedMaxY
	for _, s := range c.Series {
		for _, v := range s.Values {
			if v > maxY {
				maxY = v
			}
		}
	}
	if maxY <= 0 {
		maxY = 10.0
	} else {
		maxY = math.Ceil(maxY * 1.15)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header with title and legend
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
				Spacing:   layout.SpaceBetween,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if c.Title == "" {
						return layout.Dimensions{}
					}
					lbl := material.Label(th, unit.Sp(12), c.Title)
					lbl.Color = color.NRGBA{R: 240, G: 246, B: 252, A: 255}
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					var children []layout.FlexChild
					for _, s := range c.Series {
						seriesItem := s
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{
									Axis:      layout.Horizontal,
									Alignment: layout.Middle,
								}.Layout(gtx,
									// Dot indicator
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										dotSize := gtx.Dp(unit.Dp(6))
										shape := clip.RRect{
											Rect: image.Rect(0, 0, dotSize, dotSize),
											NW:   dotSize / 2, NE: dotSize / 2, SE: dotSize / 2, SW: dotSize / 2,
										}
										paint.FillShape(gtx.Ops, seriesItem.Color, shape.Op(gtx.Ops))
										return layout.Dimensions{Size: image.Point{X: dotSize, Y: dotSize}}
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										lbl := material.Label(th, unit.Sp(10), seriesItem.Name)
										lbl.Color = color.NRGBA{R: 139, G: 148, B: 158, A: 255}
										return lbl.Layout(gtx)
									}),
								)
							})
						}))
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		// Chart Canvas
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			w := float32(totalWidthPx)
			h := float32(chartHeightPx)

			// Canvas background
			r := gtx.Dp(unit.Dp(4))
			bgShape := clip.RRect{
				Rect: image.Rect(0, 0, totalWidthPx, chartHeightPx),
				NW:   r, NE: r, SE: r, SW: r,
			}
			paint.FillShape(gtx.Ops, color.NRGBA{R: 13, G: 17, B: 23, A: 200}, bgShape.Op(gtx.Ops))

			// Gridlines (4 horizontal divisions)
			gridColor := color.NRGBA{R: 48, G: 54, B: 61, A: 120}
			for i := 1; i <= 3; i++ {
				frac := float32(i) / 4.0
				yPos := h * (1.0 - frac)

				var path clip.Path
				path.Begin(gtx.Ops)
				path.MoveTo(f32.Pt(0, yPos))
				path.LineTo(f32.Pt(w, yPos))
				paint.FillShape(gtx.Ops, gridColor, clip.Stroke{
					Path:  path.End(),
					Width: 1.0,
				}.Op())
			}

			// Render each series line
			strokeWidth := float32(gtx.Dp(unit.Dp(2)))
			for _, s := range c.Series {
				n := len(s.Values)
				if n < 2 {
					continue
				}

				xStep := w / float32(n-1)

				var linePath clip.Path
				linePath.Begin(gtx.Ops)

				for i, v := range s.Values {
					x := float32(i) * xStep
					yFrac := float32(v / maxY)
					if yFrac > 1.0 {
						yFrac = 1.0
					}
					if yFrac < 0.0 {
						yFrac = 0.0
					}
					y := h * (1.0 - yFrac)

					if i == 0 {
						linePath.MoveTo(f32.Pt(x, y))
					} else {
						linePath.LineTo(f32.Pt(x, y))
					}
				}

				paint.FillShape(gtx.Ops, s.Color, clip.Stroke{
					Path:  linePath.End(),
					Width: strokeWidth,
				}.Op())
			}

			// Top Y-axis value label
			maxLabel := fmt.Sprintf("%.1f %s", maxY, c.Unit)
			layout.Inset{Top: unit.Dp(2), Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, unit.Sp(9), maxLabel)
				lbl.Color = color.NRGBA{R: 110, G: 118, B: 129, A: 200}
				return lbl.Layout(gtx)
			})

			return layout.Dimensions{Size: image.Point{X: totalWidthPx, Y: chartHeightPx}}
		}),
	)
}
