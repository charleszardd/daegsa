package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Card renders a modern surface container with rounded borders and padding.
type Card struct {
	Title       string
	Subtitle    string
	HeaderRight layout.Widget
	BgColor     color.NRGBA
	BorderColor color.NRGBA
	Radius      unit.Dp
	Padding     unit.Dp
}

// Layout renders the Card widget wrapping child content.
func (c Card) Layout(gtx layout.Context, th *material.Theme, w layout.Widget) layout.Dimensions {
	bg := c.BgColor
	if bg.A == 0 {
		bg = color.NRGBA{R: 22, G: 27, B: 34, A: 255} // Default #161b22
	}
	border := c.BorderColor
	if border.A == 0 {
		border = color.NRGBA{R: 48, G: 54, B: 61, A: 255} // Default #30363d
	}
	radius := c.Radius
	if radius == 0 {
		radius = unit.Dp(8)
	}
	pad := c.Padding
	if pad == 0 {
		pad = unit.Dp(16)
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			r := gtx.Dp(radius)
			rrect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Min},
				NW:   r, NE: r, SE: r, SW: r,
			}
			paint.FillShape(gtx.Ops, bg, rrect.Op(gtx.Ops))

			// Stroke border
			borderWidth := float32(gtx.Dp(unit.Dp(1)))
			paint.FillShape(gtx.Ops, border, clip.Stroke{
				Path:  rrect.Path(gtx.Ops),
				Width: borderWidth,
			}.Op())

			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    pad,
				Bottom: pad,
				Left:   pad,
				Right:  pad,
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if c.Title == "" && c.HeaderRight == nil {
							return layout.Dimensions{}
						}
						return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{
								Axis:      layout.Horizontal,
								Alignment: layout.Middle,
								Spacing:   layout.SpaceBetween,
							}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if c.Title == "" {
												return layout.Dimensions{}
											}
											lbl := material.H6(th, c.Title)
											lbl.TextSize = unit.Sp(14)
											lbl.Color = color.NRGBA{R: 240, G: 246, B: 252, A: 255}
											return lbl.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if c.Subtitle == "" {
												return layout.Dimensions{}
											}
											lbl := material.Body2(th, c.Subtitle)
											lbl.TextSize = unit.Sp(11)
											lbl.Color = color.NRGBA{R: 139, G: 148, B: 158, A: 255}
											return lbl.Layout(gtx)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if c.HeaderRight != nil {
										return c.HeaderRight(gtx)
									}
									return layout.Dimensions{}
								}),
							)
						})
					}),
					layout.Rigid(w),
				)
			})
		}),
	)
}
