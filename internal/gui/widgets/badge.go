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

// BadgeType defines the semantic badge style.
type BadgeType int

const (
	BadgeSuccess BadgeType = iota
	BadgeWarning
	BadgeDanger
	BadgeInfo
	BadgeNeutral
)

// Badge renders a styled status badge.
type Badge struct {
	Text string
	Type BadgeType
}

// Layout renders the badge.
func (b Badge) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	var bg, fg color.NRGBA

	switch b.Type {
	case BadgeSuccess:
		bg = color.NRGBA{R: 35, G: 134, B: 54, A: 45}
		fg = color.NRGBA{R: 63, G: 185, B: 80, A: 255}
	case BadgeWarning:
		bg = color.NRGBA{R: 187, G: 128, B: 9, A: 45}
		fg = color.NRGBA{R: 210, G: 153, B: 34, A: 255}
	case BadgeDanger:
		bg = color.NRGBA{R: 218, G: 54, B: 51, A: 45}
		fg = color.NRGBA{R: 248, G: 81, B: 73, A: 255}
	case BadgeInfo:
		bg = color.NRGBA{R: 31, G: 111, B: 235, A: 45}
		fg = color.NRGBA{R: 88, G: 166, B: 255, A: 255}
	default:
		bg = color.NRGBA{R: 48, G: 54, B: 61, A: 100}
		fg = color.NRGBA{R: 139, G: 148, B: 158, A: 255}
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			r := gtx.Dp(unit.Dp(10))
			rrect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Min},
				NW:   r, NE: r, SE: r, SW: r,
			}
			paint.FillShape(gtx.Ops, bg, rrect.Op(gtx.Ops))
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(2),
				Bottom: unit.Dp(2),
				Left:   unit.Dp(8),
				Right:  unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, unit.Sp(11), b.Text)
				lbl.Color = fg
				lbl.TextSize = unit.Sp(11)
				return lbl.Layout(gtx)
			})
		}),
	)
}
