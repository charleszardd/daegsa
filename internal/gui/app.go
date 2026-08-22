package gui

import (
	"fmt"
	"image"
	"image/color"
	"runtime"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/charleszardd/daegsa/internal/gui/widgets"
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
		// Top App Bar with Logo & Full-Width Header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutTopBar(gtx)
		}),

		// Middle Area: Navigation Rail + Main Content Canvas (Fills entire middle vertically)
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y

			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Left Navigation Rail (Fills full height)
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutNavRail(gtx)
				}),

				// Main Content View Canvas (Occupies remaining full width & height)
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y

					return layout.Inset{
						Top:    unit.Dp(16),
						Bottom: unit.Dp(16),
						Left:   unit.Dp(24),
						Right:  unit.Dp(24),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X

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

		// Bottom Telemetry Footer (Fills entire 100% width)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutFooter(gtx)
		}),
	)
}

func (a *App) layoutTopBar(gtx layout.Context) layout.Dimensions {
	// Force full width spanning
	gtx.Constraints.Min.X = gtx.Constraints.Max.X

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
			gtx.Constraints.Min.X = gtx.Constraints.Max.X

			return layout.Inset{
				Top:    unit.Dp(10),
				Bottom: unit.Dp(10),
				Left:   unit.Dp(20),
				Right:  unit.Dp(20),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
					Spacing:   layout.SpaceBetween,
				}.Layout(gtx,
					// Brand Logo + Title
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widgets.BrandHeader{Size: unit.Dp(30)}.Layout(gtx, a.Theme.Material)
					}),

					// Center Active Target Chip
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Stack{}.Layout(gtx,
							layout.Expanded(func(gtx layout.Context) layout.Dimensions {
								r := gtx.Dp(unit.Dp(6))
								shape := clip.RRect{
									Rect: image.Rectangle{Max: gtx.Constraints.Min},
									NW:   r, NE: r, SE: r, SW: r,
								}
								paint.FillShape(gtx.Ops, a.Theme.BgDark, shape.Op(gtx.Ops))
								paint.FillShape(gtx.Ops, a.Theme.Border, clip.Stroke{
									Path:  shape.Path(gtx.Ops),
									Width: 1.0,
								}.Op())
								return layout.Dimensions{Size: gtx.Constraints.Min}
							}),
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{
									Top:    unit.Dp(5),
									Bottom: unit.Dp(5),
									Left:   unit.Dp(12),
									Right:  unit.Dp(12),
								}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{
										Axis:      layout.Horizontal,
										Alignment: layout.Middle,
									}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											lbl := material.Label(a.Theme.Material, unit.Sp(10), a.State.Builder.Method)
											lbl.Color = a.Theme.Info
											return lbl.Layout(gtx)
										}),
										layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											lbl := material.Label(a.Theme.Material, unit.Sp(11), a.State.Builder.URL)
											lbl.Color = a.Theme.TextSecondary
											return lbl.Layout(gtx)
										}),
									)
								})
							}),
						)
					}),

					// Right Engine Status Badge
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						bType := widgets.BadgeSuccess
						statusText := "● ENGINE IDLE"
						switch a.State.RunState {
						case StateRunning:
							bType = widgets.BadgeInfo
							statusText = "● RUNNING LOAD"
						case StateDraining:
							bType = widgets.BadgeWarning
							statusText = "● DRAINING"
						case StateFailed:
							bType = widgets.BadgeDanger
							statusText = "● FAILED"
						case StateCompleted:
							bType = widgets.BadgeSuccess
							statusText = "● COMPLETED"
						}

						return widgets.Badge{
							Text: statusText,
							Type: bType,
						}.Layout(gtx, a.Theme.Material)
					}),
				)
			})
		}),
	)
}

func (a *App) layoutNavRail(gtx layout.Context) layout.Dimensions {
	w := gtx.Dp(unit.Dp(195))
	gtx.Constraints.Min.X = w
	gtx.Constraints.Max.X = w
	// Enforce 100% full vertical height spanning
	gtx.Constraints.Min.Y = gtx.Constraints.Max.Y

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rect := image.Rectangle{Max: gtx.Constraints.Min}
			paint.FillShape(gtx.Ops, a.Theme.BgSurface, clip.Rect(rect).Op())

			// Right vertical border spanning entire height
			borderW := gtx.Dp(unit.Dp(1))
			borderRect := image.Rect(rect.Max.X-borderW, 0, rect.Max.X, rect.Max.Y)
			paint.FillShape(gtx.Ops, a.Theme.Border, clip.Rect(borderRect).Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y

			return layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceBetween,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(16), Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(a.navButton(&a.btnBuilder, "Plan Studio", 0, a.State.ActiveTab == TabBuilder)),
							layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
							layout.Rigid(a.navButton(&a.btnMonitor, "Live Monitor", 1, a.State.ActiveTab == TabMonitor)),
							layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
							layout.Rigid(a.navButton(&a.btnCompare, "Compare Diff", 2, a.State.ActiveTab == TabCompare)),
							layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
							layout.Rigid(a.navButton(&a.btnDoctor, "Host Doctor", 3, a.State.ActiveTab == TabDoctor)),
						)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(16), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(a.Theme.Material, unit.Sp(10), "DAEGSA ENGINE")
								lbl.Color = a.Theme.TextMuted
								return lbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(a.Theme.Material, unit.Sp(10), "Deterministic Load Pacer")
								lbl.Color = a.Theme.TextSecondary
								return lbl.Layout(gtx)
							}),
						)
					})
				}),
			)
		}),
	)
}

func (a *App) navButton(btn *widget.Clickable, title string, iconIndex int, active bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		bg := a.Theme.BgSurface
		fg := a.Theme.TextSecondary
		iconColor := a.Theme.TextMuted

		if btn.Hovered() {
			bg = a.Theme.BgSurfaceHigh
			fg = a.Theme.TextPrimary
			iconColor = a.Theme.TextPrimary
		}

		if active {
			bg = a.Theme.BgSurfaceHigh
			fg = a.Theme.TextPrimary
			iconColor = a.Theme.Primary
		}

		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					r := gtx.Dp(unit.Dp(6))
					rect := image.Rectangle{Max: gtx.Constraints.Min}
					shape := clip.RRect{
						Rect: rect,
						NW:   r, NE: r, SE: r, SW: r,
					}
					paint.FillShape(gtx.Ops, bg, shape.Op(gtx.Ops))

					// Active left accent indicator bar (3px vertical blue stripe)
					if active {
						accentW := gtx.Dp(unit.Dp(3))
						accentRect := image.Rect(0, 0, accentW, rect.Max.Y)
						paint.FillShape(gtx.Ops, a.Theme.Primary, clip.Rect(accentRect).Op())
					}

					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    unit.Dp(10),
						Bottom: unit.Dp(10),
						Left:   unit.Dp(14),
						Right:  unit.Dp(14),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:      layout.Horizontal,
							Alignment: layout.Middle,
						}.Layout(gtx,
							// Vector Icon Indicator
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.renderNavIcon(gtx, iconIndex, iconColor)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(a.Theme.Material, unit.Sp(13), title)
								lbl.Color = fg
								return lbl.Layout(gtx)
							}),
						)
					})
				}),
			)
		})
	}
}

// renderNavIcon draws crisp vector geometric icons for each navigation tab.
func (a *App) renderNavIcon(gtx layout.Context, iconIndex int, col color.NRGBA) layout.Dimensions {
	sz := gtx.Dp(unit.Dp(14))
	s := float32(sz)

	switch iconIndex {
	case 0: // Studio - Lightning Bolt
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(f32.Pt(s*0.55, 0))
		path.LineTo(f32.Pt(s*0.15, s*0.55))
		path.LineTo(f32.Pt(s*0.48, s*0.55))
		path.LineTo(f32.Pt(s*0.40, s))
		path.LineTo(f32.Pt(s*0.85, s*0.40))
		path.LineTo(f32.Pt(s*0.52, s*0.40))
		path.Close()
		paint.FillShape(gtx.Ops, col, clip.Outline{Path: path.End()}.Op())

	case 1: // Monitor - Chart Bars
		bW := s * 0.22
		r := bW * 0.2
		// Bar 1
		p1 := clip.RRect{Rect: image.Rect(0, int(s*0.45), int(bW), int(s)), NW: int(r), NE: int(r), SE: int(r), SW: int(r)}
		paint.FillShape(gtx.Ops, col, p1.Op(gtx.Ops))
		// Bar 2
		p2 := clip.RRect{Rect: image.Rect(int(s*0.38), int(s*0.15), int(s*0.38+bW), int(s)), NW: int(r), NE: int(r), SE: int(r), SW: int(r)}
		paint.FillShape(gtx.Ops, col, p2.Op(gtx.Ops))
		// Bar 3
		p3 := clip.RRect{Rect: image.Rect(int(s*0.76), int(s*0.30), int(s), int(s)), NW: int(r), NE: int(r), SE: int(r), SW: int(r)}
		paint.FillShape(gtx.Ops, col, p3.Op(gtx.Ops))

	case 2: // Compare - Delta / Scale Balance
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(f32.Pt(s*0.5, s*0.08))
		path.LineTo(f32.Pt(s*0.92, s*0.88))
		path.LineTo(f32.Pt(s*0.08, s*0.88))
		path.Close()
		paint.FillShape(gtx.Ops, col, clip.Stroke{
			Path:  path.End(),
			Width: float32(gtx.Dp(unit.Dp(1.5))),
		}.Op())

	case 3: // Doctor - Pulse Heartbeat Plus
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(f32.Pt(0, s*0.5))
		path.LineTo(f32.Pt(s*0.25, s*0.5))
		path.LineTo(f32.Pt(s*0.40, s*0.15))
		path.LineTo(f32.Pt(s*0.60, s*0.85))
		path.LineTo(f32.Pt(s*0.75, s*0.5))
		path.LineTo(f32.Pt(s, s*0.5))
		paint.FillShape(gtx.Ops, col, clip.Stroke{
			Path:  path.End(),
			Width: float32(gtx.Dp(unit.Dp(1.5))),
		}.Op())
	}

	return layout.Dimensions{Size: image.Point{X: sz, Y: sz}}
}

func (a *App) layoutFooter(gtx layout.Context) layout.Dimensions {
	// Force full width spanning
	gtx.Constraints.Min.X = gtx.Constraints.Max.X

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
			gtx.Constraints.Min.X = gtx.Constraints.Max.X

			return layout.Inset{
				Top:    unit.Dp(6),
				Bottom: unit.Dp(6),
				Left:   unit.Dp(20),
				Right:  unit.Dp(20),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
					Spacing:   layout.SpaceBetween,
				}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(a.Theme.Material, unit.Sp(11), fmt.Sprintf("Target: %s  [%s]", a.State.Builder.URL, a.State.Builder.Model))
						lbl.Color = a.Theme.TextMuted
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						info := fmt.Sprintf("Goroutines: %d  •  RAM: %.1f MB  •  Cores: %d  •  Platform: %s/%s", runtime.NumGoroutine(), allocMB, runtime.NumCPU(), runtime.GOOS, runtime.GOARCH)
						lbl := material.Label(a.Theme.Material, unit.Sp(10), info)
						lbl.Color = a.Theme.TextMuted
						return lbl.Layout(gtx)
					}),
				)
			})
		}),
	)
}
