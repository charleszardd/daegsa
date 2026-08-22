package views

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/charleszardd/daegsa/internal/gui"
	"github.com/charleszardd/daegsa/internal/gui/widgets"
)

// CompareView manages UI for comparing baseline vs candidate test reports.
type CompareView struct {
	State *gui.State

	baselinePathEd  widget.Editor
	candidatePathEd widget.Editor
	compareBtn      widget.Clickable

	listState layout.List
}

// NewCompareView creates a new CompareView.
func NewCompareView(s *gui.State) *CompareView {
	return &CompareView{
		State:     s,
		listState: layout.List{Axis: layout.Vertical},
	}
}

// Layout renders the comparison view.
func (v *CompareView) Layout(gtx layout.Context, th *gui.Theme) layout.Dimensions {
	if v.compareBtn.Clicked(gtx) {
		v.State.Compare.BaselinePath = v.baselinePathEd.Text()
		v.State.Compare.CandidatePath = v.candidatePathEd.Text()
		v.State.RunComparison()
	}

	res := v.State.Compare.Result

	items := []layout.Widget{
		// Header
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
				Spacing:   layout.SpaceBetween,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.H5(th.Material, "Report Comparison & Regression Studio")
					lbl.Color = th.TextPrimary
					lbl.TextSize = unit.Sp(18)
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th.Material, &v.compareBtn, "Compare Reports")
					btn.Background = th.Primary
					btn.Color = th.TextPrimary
					btn.TextSize = unit.Sp(12)
					btn.Inset = layout.UniformInset(unit.Dp(8))
					return btn.Layout(gtx)
				}),
			)
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Input File Selectors Card
		func(gtx layout.Context) layout.Dimensions {
			return widgets.Card{
				Title:    "Report Files",
				Subtitle: "Specify baseline and candidate JSON report paths",
			}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
						return v.renderPathInput(gtx, th, "Baseline Report Path", &v.baselinePathEd, "e.g. baseline.json")
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
					layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
						return v.renderPathInput(gtx, th, "Candidate Report Path", &v.candidatePathEd, "e.g. candidate.json")
					}),
				)
			})
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Error message if any
		func(gtx layout.Context) layout.Dimensions {
			if v.State.Compare.ErrorMessage == "" {
				return layout.Dimensions{}
			}
			return widgets.Card{
				BorderColor: th.Danger,
			}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th.Material, v.State.Compare.ErrorMessage)
				lbl.Color = th.DangerText
				return lbl.Layout(gtx)
			})
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Comparison Table Card
		func(gtx layout.Context) layout.Dimensions {
			if res == nil {
				return widgets.Card{}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th.Material, "Select report files above and click 'Compare Reports' to view regression analysis.")
					lbl.Color = th.TextSecondary
					return lbl.Layout(gtx)
				})
			}

			return widgets.Card{
				Title: "Metric Regression Analysis",
			}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
				var rows []layout.FlexChild

				// Table Header
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(0.3, v.tableHeaderCell(th, "Metric Name")),
						layout.Flexed(0.2, v.tableHeaderCell(th, "Baseline")),
						layout.Flexed(0.2, v.tableHeaderCell(th, "Candidate")),
						layout.Flexed(0.15, v.tableHeaderCell(th, "Abs Delta")),
						layout.Flexed(0.15, v.tableHeaderCell(th, "% Delta")),
					)
				}))

				rows = append(rows, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))

				// Table Data Rows
				for _, d := range res.Deltas {
					deltaItem := d
					rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						pctStr := "N/A"
						if deltaItem.PercentageAvailable {
							pctStr = fmt.Sprintf("%+.2f%%", deltaItem.Percentage)
						}

						deltaColor := th.TextSecondary
						if deltaItem.PercentageAvailable && deltaItem.Percentage > 10.0 {
							deltaColor = th.WarningText
						}
						if deltaItem.PercentageAvailable && deltaItem.Percentage > 25.0 {
							deltaColor = th.DangerText
						}

						return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(0.3, v.tableCell(th, deltaItem.Name, th.TextPrimary)),
								layout.Flexed(0.2, v.tableCell(th, fmt.Sprintf("%.2f", deltaItem.Baseline), th.TextSecondary)),
								layout.Flexed(0.2, v.tableCell(th, fmt.Sprintf("%.2f", deltaItem.Candidate), th.TextSecondary)),
								layout.Flexed(0.15, v.tableCell(th, fmt.Sprintf("%+.2f", deltaItem.Absolute), th.TextSecondary)),
								layout.Flexed(0.15, v.tableCell(th, pctStr, deltaColor)),
							)
						})
					}))
				}

				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			})
		},
	}

	return v.listState.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		return items[index](gtx)
	})
}

func (v *CompareView) renderPathInput(gtx layout.Context, th *gui.Theme, label string, ed *widget.Editor, hint string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(th.Material, unit.Sp(11), label)
			lbl.Color = th.TextSecondary
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			med := material.Editor(th.Material, ed, hint)
			med.TextSize = unit.Sp(12)
			med.Color = th.TextPrimary

			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					r := gtx.Dp(unit.Dp(4))
					shape := clip.RRect{
						Rect: image.Rectangle{Max: gtx.Constraints.Min},
						NW:   r, NE: r, SE: r, SW: r,
					}
					paint.FillShape(gtx.Ops, color.NRGBA{R: 13, G: 17, B: 23, A: 255}, shape.Op(gtx.Ops))
					paint.FillShape(gtx.Ops, th.Border, clip.Stroke{
						Path:  shape.Path(gtx.Ops),
						Width: 1.0,
					}.Op())
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    unit.Dp(6),
						Bottom: unit.Dp(6),
						Left:   unit.Dp(8),
						Right:  unit.Dp(8),
					}.Layout(gtx, med.Layout)
				}),
			)
		}),
	)
}

func (v *CompareView) tableHeaderCell(th *gui.Theme, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(th.Material, unit.Sp(11), text)
		lbl.Color = th.TextMuted
		return lbl.Layout(gtx)
	}
}

func (v *CompareView) tableCell(th *gui.Theme, text string, col color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(th.Material, text)
		lbl.Color = col
		lbl.TextSize = unit.Sp(12)
		return lbl.Layout(gtx)
	}
}
