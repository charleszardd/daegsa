package views

import (
	"context"
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/charleszardd/daegsa/internal/doctor"
	"github.com/charleszardd/daegsa/internal/gui"
	"github.com/charleszardd/daegsa/internal/gui/widgets"
)

// DoctorView manages UI for host readiness diagnostics.
type DoctorView struct {
	State *gui.State

	diagnoseBtn widget.Clickable
	listState   layout.List
}

// NewDoctorView constructs a DoctorView.
func NewDoctorView(s *gui.State) *DoctorView {
	return &DoctorView{
		State:     s,
		listState: layout.List{Axis: layout.Vertical},
	}
}

// Layout renders the doctor diagnostics view.
func (v *DoctorView) Layout(gtx layout.Context, th *gui.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X

	if v.diagnoseBtn.Clicked(gtx) {
		v.State.RunDiagnostics(context.Background())
	}

	doc := v.State.Doctor

	items := []layout.Widget{
		// Header Action Bar
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X

			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
				Spacing:   layout.SpaceBetween,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.H5(th.Material, "System Readiness & Diagnostics (Doctor)")
					lbl.Color = th.TextPrimary
					lbl.TextSize = unit.Sp(18)
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th.Material, &v.diagnoseBtn, "Run Diagnostics")
					btn.Background = th.Primary
					btn.Color = th.TextPrimary
					btn.TextSize = unit.Sp(12)
					btn.Inset = layout.UniformInset(unit.Dp(8))
					return btn.Layout(gtx)
				}),
			)
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Diagnostic Results or Empty Prompt
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X

			if doc == nil {
				return widgets.Card{
					Title:    "Host Diagnostics Readiness",
					Subtitle: "Timer precision, DNS resolution, TLS certs, and socket headroom",
				}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X

					return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body1(th.Material, "No diagnostics executed yet.")
								lbl.Color = th.TextSecondary
								return lbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th.Material, "Click 'Run Diagnostics' above to inspect OS timer resolution, DNS latency, socket headroom, and system limits.")
								lbl.Color = th.TextMuted
								return lbl.Layout(gtx)
							}),
						)
					})
				})
			}

			overallType := widgets.BadgeSuccess
			if doc.OverallStatus == doctor.StatusWarn {
				overallType = widgets.BadgeWarning
			} else if doc.OverallStatus == doctor.StatusFail {
				overallType = widgets.BadgeDanger
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				// Overall Status Card
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X

					return widgets.Card{
						Title: "Overall Host Readiness",
						HeaderRight: func(gtx layout.Context) layout.Dimensions {
							return widgets.Badge{
								Text: string(doc.OverallStatus),
								Type: overallType,
							}.Layout(gtx, th.Material)
						},
					}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X

						return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
							layout.Rigid(v.renderInfoItem(th, "Operating System", fmt.Sprintf("%s / %s", doc.System.OS, doc.System.Arch))),
							layout.Rigid(v.renderInfoItem(th, "CPU Cores", fmt.Sprintf("%d logical (%d GOMAXPROCS)", doc.System.NumCPU, doc.System.GOMAXPROCS))),
							layout.Rigid(v.renderInfoItem(th, "Go Version", doc.System.GoVersion)),
							layout.Rigid(v.renderInfoItem(th, "Mem Allocated", fmt.Sprintf("%.2f MB", float64(doc.System.MemoryAlloc)/(1024*1024)))),
						)
					})
				}),

				layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),

				// Check Items
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					var checkRows []layout.FlexChild

					for _, c := range doc.Checks {
						checkItem := c
						bType := widgets.BadgeSuccess
						if checkItem.Status == doctor.StatusWarn {
							bType = widgets.BadgeWarning
						} else if checkItem.Status == doctor.StatusFail {
							bType = widgets.BadgeDanger
						}

						checkRows = append(checkRows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X

							return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return widgets.Card{
									Title: checkItem.Name,
									HeaderRight: func(gtx layout.Context) layout.Dimensions {
										return widgets.Badge{
											Text: string(checkItem.Status),
											Type: bType,
										}.Layout(gtx, th.Material)
									},
								}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X

									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											lbl := material.Body2(th.Material, checkItem.Summary)
											lbl.Color = th.TextPrimary
											return lbl.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if checkItem.Suggestion == "" {
												return layout.Dimensions{}
											}
											return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												lbl := material.Body2(th.Material, fmt.Sprintf("Suggestion: %s", checkItem.Suggestion))
												lbl.Color = th.TextSecondary
												return lbl.Layout(gtx)
											})
										}),
									)
								})
							})
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, checkRows...)
				}),
			)
		},
	}

	return v.listState.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		return items[index](gtx)
	})
}

func (v *DoctorView) renderInfoItem(th *gui.Theme, label, val string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th.Material, unit.Sp(10), label)
				lbl.Color = th.TextSecondary
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th.Material, val)
				lbl.Color = th.TextPrimary
				lbl.TextSize = unit.Sp(12)
				return lbl.Layout(gtx)
			}),
		)
	}
}
