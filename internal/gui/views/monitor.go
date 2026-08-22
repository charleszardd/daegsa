package views

import (
	"context"
	"fmt"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/charleszardd/daegsa/internal/gui"
	"github.com/charleszardd/daegsa/internal/gui/widgets"
)

// MonitorView manages live load test telemetry and execution controls.
type MonitorView struct {
	State *gui.State

	stopBtn  widget.Clickable
	abortBtn widget.Clickable
	retryBtn widget.Clickable

	listState layout.List
}

// NewMonitorView constructs a MonitorView.
func NewMonitorView(s *gui.State) *MonitorView {
	return &MonitorView{
		State:     s,
		listState: layout.List{Axis: layout.Vertical},
	}
}

// Layout renders the live telemetry view.
func (v *MonitorView) Layout(gtx layout.Context, th *gui.Theme) layout.Dimensions {
	if v.stopBtn.Clicked(gtx) {
		v.State.StopGracefully()
	}
	if v.abortBtn.Clicked(gtx) {
		v.State.AbortExecution()
	}
	if v.retryBtn.Clicked(gtx) {
		_ = v.State.StartExecution(context.Background())
	}

	snap := v.State.Telemetry

	items := []layout.Widget{
		// Top Control Bar
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
				Spacing:   layout.SpaceBetween,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							stateBadgeType := widgets.BadgeNeutral
							switch v.State.RunState {
							case gui.StateRunning:
								stateBadgeType = widgets.BadgeInfo
							case gui.StateDraining:
								stateBadgeType = widgets.BadgeWarning
							case gui.StateCompleted:
								stateBadgeType = widgets.BadgeSuccess
							case gui.StateFailed:
								stateBadgeType = widgets.BadgeDanger
							}
							return widgets.Badge{
								Text: v.State.RunState.String(),
								Type: stateBadgeType,
							}.Layout(gtx, th.Material)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							durText := fmt.Sprintf("Elapsed: %s / %s", snap.Elapsed.Round(100), snap.Duration)
							lbl := material.Label(th.Material, unit.Sp(13), durText)
							lbl.Color = th.TextSecondary
							return lbl.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if v.State.RunState == gui.StateRunning {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.Button(th.Material, &v.stopBtn, "Stop Gracefully")
								btn.Background = th.Warning
								btn.Color = th.TextPrimary
								btn.TextSize = unit.Sp(11)
								btn.Inset = layout.UniformInset(unit.Dp(6))
								return btn.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.Button(th.Material, &v.abortBtn, "Abort")
								btn.Background = th.Danger
								btn.Color = th.TextPrimary
								btn.TextSize = unit.Sp(11)
								btn.Inset = layout.UniformInset(unit.Dp(6))
								return btn.Layout(gtx)
							}),
						)
					}
					return material.Button(th.Material, &v.retryBtn, "Re-run Test").Layout(gtx)
				}),
			)
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Top 4 Metrics Cards
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Card 1: Target / Achieved RPS
				layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
					return v.renderStatCard(gtx, th, "Target / Start Rate",
						fmt.Sprintf("%.1f / %.1f", snap.TargetRPS, snap.AchievedRPS),
						"req/s arrival pacing", th.Primary)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				// Card 2: Completed Throughput
				layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
					return v.renderStatCard(gtx, th, "Completed Throughput",
						fmt.Sprintf("%.1f req/s", snap.CompletedRPS),
						fmt.Sprintf("%d total requests", snap.CompletedTotal), th.SuccessText)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				// Card 3: In-Flight Requests
				layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
					return widgets.Card{}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
						return widgets.Gauge{
							Label:   "In-Flight Requests",
							Current: snap.InFlight,
							Max:     snap.MaxInFlight,
						}.Layout(gtx, th.Material)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				// Card 4: Error Rate
				layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
					statColor := th.SuccessText
					if snap.ErrorRate > 0.01 {
						statColor = th.DangerText
					}
					return v.renderStatCard(gtx, th, "Error Rate",
						fmt.Sprintf("%.2f%%", snap.ErrorRate*100),
						fmt.Sprintf("%d dropped / %d errors", snap.DroppedTotal, snap.ErrorCount), statColor)
				}),
			)
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Dual Real-Time Charts
		func(gtx layout.Context) layout.Dimensions {
			// Extract time series
			var targetRPSVals, completedRPSVals []float64
			var p50Vals, p95Vals, p99Vals []float64

			for _, pt := range snap.TimeSeries {
				targetRPSVals = append(targetRPSVals, pt.TargetRPS)
				completedRPSVals = append(completedRPSVals, pt.CompletedRPS)
				p50Vals = append(p50Vals, pt.P50MS)
				p95Vals = append(p95Vals, pt.P95MS)
				p99Vals = append(p99Vals, pt.P99MS)
			}

			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Left Chart: Throughput
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return widgets.Card{
						Title: "Throughput (RPS)",
					}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
						return widgets.LineChart{
							Unit: "req/s",
							Series: []widgets.ChartSeries{
								{Name: "Target", Color: th.TextSecondary, Values: targetRPSVals},
								{Name: "Completed", Color: th.SuccessText, Values: completedRPSVals},
							},
						}.Layout(gtx, th.Material)
					})
				}),

				layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),

				// Right Chart: Latency
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return widgets.Card{
						Title: "Latency Percentiles (ms)",
					}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
						return widgets.LineChart{
							Unit: "ms",
							Series: []widgets.ChartSeries{
								{Name: "p50", Color: th.Info, Values: p50Vals},
								{Name: "p95", Color: th.WarningText, Values: p95Vals},
								{Name: "p99", Color: th.DangerText, Values: p99Vals},
							},
						}.Layout(gtx, th.Material)
					})
				}),
			)
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Bottom Panels: Latency Summary & Threshold Assertions
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Latency breakdown card
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return widgets.Card{
						Title: "Latency Distribution Summary",
					}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
									layout.Rigid(v.renderMetricItem(th, "p50 (Median)", fmt.Sprintf("%.2f ms", snap.P50MS))),
									layout.Rigid(v.renderMetricItem(th, "p90", fmt.Sprintf("%.2f ms", snap.P90MS))),
									layout.Rigid(v.renderMetricItem(th, "p95", fmt.Sprintf("%.2f ms", snap.P95MS))),
									layout.Rigid(v.renderMetricItem(th, "p99", fmt.Sprintf("%.2f ms", snap.P99MS))),
									layout.Rigid(v.renderMetricItem(th, "Max", fmt.Sprintf("%.2f ms", snap.MaxMS))),
								)
							}),
						)
					})
				}),

				layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),

				// Threshold Results
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return widgets.Card{
						Title: "Threshold Assertions",
					}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
						if len(snap.ThresholdResults) == 0 {
							lbl := material.Body2(th.Material, "No threshold assertions defined.")
							lbl.Color = th.TextSecondary
							return lbl.Layout(gtx)
						}

						var rows []layout.FlexChild
						for _, tr := range snap.ThresholdResults {
							item := tr
							rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								bType := widgets.BadgeSuccess
								bText := "PASS"
								if !item.Passed {
									bType = widgets.BadgeDanger
									bText = "FAIL"
								}
								return layout.Flex{
									Axis:      layout.Horizontal,
									Alignment: layout.Middle,
									Spacing:   layout.SpaceBetween,
								}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body2(th.Material, item.Expression)
										lbl.Color = th.TextPrimary
										return lbl.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return widgets.Badge{Text: bText, Type: bType}.Layout(gtx, th.Material)
									}),
								)
							}))
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
					})
				}),
			)
		},
	}

	return v.listState.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		return items[index](gtx)
	})
}

func (v *MonitorView) renderStatCard(gtx layout.Context, th *gui.Theme, title, value, subtitle string, valColor color.NRGBA) layout.Dimensions {
	return widgets.Card{}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th.Material, unit.Sp(11), title)
				lbl.Color = th.TextSecondary
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.H6(th.Material, value)
				lbl.Color = valColor
				lbl.TextSize = unit.Sp(16)
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th.Material, unit.Sp(10), subtitle)
				lbl.Color = th.TextMuted
				return lbl.Layout(gtx)
			}),
		)
	})
}

func (v *MonitorView) renderMetricItem(th *gui.Theme, label, val string) layout.Widget {
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
				lbl.TextSize = unit.Sp(13)
				return lbl.Layout(gtx)
			}),
		)
	}
}
