package gui

import (
	"fmt"
	"image"
	"runtime"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// View defines the common interface for top-level workspace views.
type View interface {
	Layout(gtx layout.Context, th *Theme) layout.Dimensions
}

// App coordinates the window lifecycle, navigation rail, and active view rendering.
type App struct {
	Window *app.Window
	Theme  *Theme
	State  *State

	// Navigation Clickables
	btnBuilder widget.Clickable
	btnMonitor widget.Clickable
	btnCompare widget.Clickable
	btnDoctor  widget.Clickable

	// Views
	BuilderView View
	MonitorView View
	CompareView View
	DoctorView  View
}

// NewApp creates a configured desktop application instance.
func NewApp(w *app.Window) *App {
	th := NewTheme()
	state := NewState(func() {
		if w != nil {
			w.Invalidate()
		}
	})

	return &App{
		Window: w,
		Theme:  th,
		State:  state,
	}
}

// SetViews binds the instantiated view controllers to the app shell.
func (a *App) SetViews(builder, monitor, compare, doctor View) {
	a.BuilderView = builder
	a.MonitorView = monitor
	a.CompareView = compare
	a.DoctorView = doctor
}

// Run executes the Gio frame event loop.
func (a *App) Run() error {
	var ops op.Ops

	for {
		switch e := a.Window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// layout renders the entire application window UI hierarchy.
func (a *App) layout(gtx layout.Context) layout.Dimensions {
	// Handle navigation clicks
	if a.btnBuilder.Clicked(gtx) {
		a.State.ActiveTab = TabBuilder
	}
	if a.btnMonitor.Clicked(gtx) {
		a.State.ActiveTab = TabMonitor
	}
	if a.btnCompare.Clicked(gtx) {
		a.State.ActiveTab = TabCompare
	}
	if a.btnDoctor.Clicked(gtx) {
		a.State.ActiveTab = TabDoctor
	}

	// Paint global background
	paint.Fill(gtx.Ops, a.Theme.BgDark)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Top App Bar
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutTopBar(gtx)
		}),

		// Middle Area: Navigation Rail + Main Content Canvas
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Left Navigation Rail
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutNavRail(gtx)
				}),

				// Main Content View Canvas
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    unit.Dp(16),
						Bottom: unit.Dp(16),
						Left:   unit.Dp(20),
						Right:  unit.Dp(20),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						switch a.State.ActiveTab {
						case TabBuilder:
							if a.BuilderView != nil {
								return a.BuilderView.Layout(gtx, a.Theme)
							}
						case TabMonitor:
							if a.MonitorView != nil {
								return a.MonitorView.Layout(gtx, a.Theme)
							}
						case TabCompare:
							if a.CompareView != nil {
								return a.CompareView.Layout(gtx, a.Theme)
							}
						case TabDoctor:
							if a.DoctorView != nil {
								return a.DoctorView.Layout(gtx, a.Theme)
							}
						}
						return layout.Dimensions{}
					})
				}),
			)
		}),

		// Bottom Telemetry Footer
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutFooter(gtx)
		}),
	)
}

func (a *App) layoutTopBar(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rect := image.Rectangle{Max: gtx.Constraints.Min}
			paint.FillShape(gtx.Ops, a.Theme.BgSurface, clip.Rect(rect).Op())

			// Bottom border
			borderH := gtx.Dp(unit.Dp(1))
			borderRect := image.Rect(0, rect.Max.Y-borderH, rect.Max.X, rect.Max.Y)
			paint.FillShape(gtx.Ops, a.Theme.Border, clip.Rect(borderRect).Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(10),
				Bottom: unit.Dp(10),
				Left:   unit.Dp(16),
				Right:  unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
					Spacing:   layout.SpaceBetween,
				}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.H6(a.Theme.Material, "DAEGSA")
								lbl.Color = a.Theme.Primary
								lbl.TextSize = unit.Sp(16)
								return lbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(a.Theme.Material, "Studio v0.1.0-dev")
								lbl.Color = a.Theme.TextSecondary
								lbl.TextSize = unit.Sp(12)
								return lbl.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						statusText := "Ready"
						statusColor := a.Theme.SuccessText
						if a.State.RunState == StateRunning {
							statusText = "Running Load Test"
							statusColor = a.Theme.Info
						}
						lbl := material.Body2(a.Theme.Material, fmt.Sprintf("Engine: %s", statusText))
						lbl.Color = statusColor
						lbl.TextSize = unit.Sp(12)
						return lbl.Layout(gtx)
					}),
				)
			})
		}),
	)
}

func (a *App) layoutNavRail(gtx layout.Context) layout.Dimensions {
	w := gtx.Dp(unit.Dp(170))
	gtx.Constraints.Min.X = w
	gtx.Constraints.Max.X = w

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rect := image.Rectangle{Max: gtx.Constraints.Min}
			paint.FillShape(gtx.Ops, a.Theme.BgSurface, clip.Rect(rect).Op())

			// Right vertical border
			borderW := gtx.Dp(unit.Dp(1))
			borderRect := image.Rect(rect.Max.X-borderW, 0, rect.Max.X, rect.Max.Y)
			paint.FillShape(gtx.Ops, a.Theme.Border, clip.Rect(borderRect).Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(a.navButton(&a.btnBuilder, "Plan Studio", a.State.ActiveTab == TabBuilder)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(a.navButton(&a.btnMonitor, "Live Monitor", a.State.ActiveTab == TabMonitor)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(a.navButton(&a.btnCompare, "Compare Diff", a.State.ActiveTab == TabCompare)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(a.navButton(&a.btnDoctor, "Host Doctor", a.State.ActiveTab == TabDoctor)),
				)
			})
		}),
	)
}

func (a *App) navButton(btn *widget.Clickable, title string, active bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		bg := a.Theme.BgSurface
		fg := a.Theme.TextSecondary
		if active {
			bg = a.Theme.BgSurfaceHigh
			fg = a.Theme.TextPrimary
		}

		b := material.Button(a.Theme.Material, btn, title)
		b.Background = bg
		b.Color = fg
		b.TextSize = unit.Sp(13)
		b.Inset = layout.Inset{
			Top:    unit.Dp(10),
			Bottom: unit.Dp(10),
			Left:   unit.Dp(12),
			Right:  unit.Dp(12),
		}
		return b.Layout(gtx)
	}
}

func (a *App) layoutFooter(gtx layout.Context) layout.Dimensions {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	allocMB := float64(mem.Alloc) / (1024 * 1024)

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rect := image.Rectangle{Max: gtx.Constraints.Min}
			paint.FillShape(gtx.Ops, a.Theme.BgSurface, clip.Rect(rect).Op())

			// Top border
			borderH := gtx.Dp(unit.Dp(1))
			borderRect := image.Rect(0, 0, rect.Max.X, borderH)
			paint.FillShape(gtx.Ops, a.Theme.Border, clip.Rect(borderRect).Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(6),
				Bottom: unit.Dp(6),
				Left:   unit.Dp(16),
				Right:  unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
					Spacing:   layout.SpaceBetween,
				}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(a.Theme.Material, unit.Sp(11), fmt.Sprintf("Target: %s", a.State.Builder.URL))
						lbl.Color = a.Theme.TextMuted
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						info := fmt.Sprintf("Goroutines: %d | Memory: %.1f MB | Cores: %d", runtime.NumGoroutine(), allocMB, runtime.NumCPU())
						lbl := material.Label(a.Theme.Material, unit.Sp(11), info)
						lbl.Color = a.Theme.TextMuted
						return lbl.Layout(gtx)
					}),
				)
			})
		}),
	)
}
