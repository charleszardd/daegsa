package views

import (
	"context"
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

// BuilderView manages layout and interactions for the Test Studio / Plan Builder.
type BuilderView struct {
	State *gui.State

	// Form Editors
	urlEditor      widget.Editor
	rateEditor     widget.Editor
	usersEditor    widget.Editor
	durationEditor widget.Editor
	maxInFlightEd  widget.Editor
	thinkTimeEd    widget.Editor
	yamlEditor     widget.Editor

	// Controls
	modelEnum        widget.Enum
	methodEnum       widget.Enum
	allowDestructive widget.Bool
	validateBtn      widget.Clickable
	startBtn         widget.Clickable

	listState layout.List
	inited    bool
}

// NewBuilderView creates a new Plan Builder view.
func NewBuilderView(s *gui.State) *BuilderView {
	v := &BuilderView{
		State:     s,
		listState: layout.List{Axis: layout.Vertical},
	}
	return v
}

func (v *BuilderView) initDefaults() {
	if v.inited {
		return
	}
	v.inited = true

	v.urlEditor.SetText(v.State.Builder.URL)
	v.rateEditor.SetText(v.State.Builder.Rate)
	v.usersEditor.SetText(v.State.Builder.Users)
	v.durationEditor.SetText(v.State.Builder.Duration)
	v.maxInFlightEd.SetText(v.State.Builder.MaxInFlight)
	v.thinkTimeEd.SetText(v.State.Builder.ThinkTime)
	v.yamlEditor.SetText(v.State.Builder.ConfigYAML)

	v.modelEnum.Value = v.State.Builder.Model
	v.methodEnum.Value = v.State.Builder.Method
	v.allowDestructive.Value = v.State.Builder.AllowDestructive
}

// Layout renders the builder view.
func (v *BuilderView) Layout(gtx layout.Context, th *gui.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	v.initDefaults()

	// Handle button events
	if v.validateBtn.Clicked(gtx) {
		v.syncToState()
		_ = v.State.ValidateCurrentPlan(context.Background())
	}
	if v.startBtn.Clicked(gtx) {
		v.syncToState()
		_ = v.State.StartExecution(context.Background())
	}

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
					lbl := material.H5(th.Material, "Test Plan Studio")
					lbl.Color = th.TextPrimary
					lbl.TextSize = unit.Sp(18)
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(th.Material, &v.validateBtn, "Preflight Validate")
							btn.Background = th.BgSurfaceHigh
							btn.Color = th.TextPrimary
							btn.TextSize = unit.Sp(12)
							btn.Inset = layout.UniformInset(unit.Dp(8))
							return btn.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(th.Material, &v.startBtn, "Start Load Test")
							btn.Background = th.Success
							btn.Color = th.TextPrimary
							btn.TextSize = unit.Sp(12)
							btn.Inset = layout.UniformInset(unit.Dp(8))
							return btn.Layout(gtx)
						}),
					)
				}),
			)
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Validation Result Banner (if available)
		func(gtx layout.Context) layout.Dimensions {
			if v.State.Builder.ValidationStatus == "" {
				return layout.Dimensions{}
			}
			gtx.Constraints.Min.X = gtx.Constraints.Max.X

			badgeType := widgets.BadgeSuccess
			if v.State.Builder.ValidationStatus != "PASS" {
				badgeType = widgets.BadgeDanger
			}

			return widgets.Card{
				Title:       "Preflight Status",
				BorderColor: th.Border,
				HeaderRight: func(gtx layout.Context) layout.Dimensions {
					return widgets.Badge{
						Text: v.State.Builder.ValidationStatus,
						Type: badgeType,
					}.Layout(gtx, th.Material)
				},
			}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th.Material, v.State.Builder.ValidationMessage)
				lbl.Color = th.TextSecondary
				return lbl.Layout(gtx)
			})
		},

		layout.Spacer{Height: unit.Dp(16)}.Layout,

		// Main Config Form & YAML Split
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X

			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Left Panel: Quick Form
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return widgets.Card{
						Title:    "Workload Parameters",
						Subtitle: "Configure target endpoint, traffic model, and rate limits",
					}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							// URL
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return v.renderField(gtx, th, "Target URL", &v.urlEditor)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							// Method & Model Selectors
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												lbl := material.Label(th.Material, unit.Sp(11), "Workload Model")
												lbl.Color = th.TextSecondary
												return lbl.Layout(gtx)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
													layout.Rigid(material.RadioButton(th.Material, &v.modelEnum, "open", "Open (RPS)").Layout),
													layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
													layout.Rigid(material.RadioButton(th.Material, &v.modelEnum, "closed", "Closed (VU)").Layout),
												)
											}),
										)
									}),
								)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							// Rate / Users Row
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
										return v.renderField(gtx, th, "Target Rate (req/s)", &v.rateEditor)
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
									layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
										return v.renderField(gtx, th, "Virtual Users (VU)", &v.usersEditor)
									}),
								)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							// Duration & Max In Flight Row
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
										return v.renderField(gtx, th, "Duration (e.g. 30s, 2m)", &v.durationEditor)
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
									layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
										return v.renderField(gtx, th, "Max In-Flight", &v.maxInFlightEd)
									}),
								)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							// Destructive Safety Checkbox
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								cb := material.CheckBox(th.Material, &v.allowDestructive, "Allow Destructive Methods (POST/PUT/DELETE)")
								cb.Color = th.WarningText
								return cb.Layout(gtx)
							}),
						)
					})
				}),

				layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),

				// Right Panel: Full YAML Editor
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return widgets.Card{
						Title:    "YAML Configuration Manifest",
						Subtitle: "Edit full scenario, multi-step workflows, and assertions",
					}.Layout(gtx, th.Material, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(th.Material, &v.yamlEditor, "Enter YAML configuration...")
								ed.TextSize = unit.Sp(12)
								ed.Color = th.TextPrimary

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
											Top:    unit.Dp(8),
											Bottom: unit.Dp(8),
											Left:   unit.Dp(8),
											Right:  unit.Dp(8),
										}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(280))
											return ed.Layout(gtx)
										})
									}),
								)
							}),
						)
					})
				}),
			)
		},
	}

	return v.listState.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		return items[index](gtx)
	})
}

func (v *BuilderView) renderField(gtx layout.Context, th *gui.Theme, label string, ed *widget.Editor) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(th.Material, unit.Sp(11), label)
			lbl.Color = th.TextSecondary
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			med := material.Editor(th.Material, ed, "")
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

func (v *BuilderView) syncToState() {
	v.State.Builder.URL = v.urlEditor.Text()
	v.State.Builder.Rate = v.rateEditor.Text()
	v.State.Builder.Users = v.usersEditor.Text()
	v.State.Builder.Duration = v.durationEditor.Text()
	v.State.Builder.MaxInFlight = v.maxInFlightEd.Text()
	v.State.Builder.ThinkTime = v.thinkTimeEd.Text()
	v.State.Builder.ConfigYAML = v.yamlEditor.Text()
	v.State.Builder.Model = v.modelEnum.Value
	v.State.Builder.AllowDestructive = v.allowDestructive.Value
}
