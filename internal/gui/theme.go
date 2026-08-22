package gui

import (
	"image/color"

	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Theme holds the global color palette and Material theme instance for DAEGSA Studio.
type Theme struct {
	Material *material.Theme

	// Palette
	BgDark        color.NRGBA
	BgSurface     color.NRGBA
	BgSurfaceHigh color.NRGBA
	Border        color.NRGBA
	BorderFocus   color.NRGBA

	TextPrimary   color.NRGBA
	TextSecondary color.NRGBA
	TextMuted     color.NRGBA

	Primary     color.NRGBA
	PrimaryDark color.NRGBA
	Accent      color.NRGBA

	Success     color.NRGBA
	SuccessText color.NRGBA
	Warning     color.NRGBA
	WarningText color.NRGBA
	Danger      color.NRGBA
	DangerText  color.NRGBA
	Info        color.NRGBA

	// Spacings
	PaddingXS unit.Dp
	PaddingSM unit.Dp
	PaddingMD unit.Dp
	PaddingLG unit.Dp
	PaddingXL unit.Dp

	// Radiuses
	RadiusSM unit.Dp
	RadiusMD unit.Dp
	RadiusLG unit.Dp
}

// NewTheme constructs a dark theme configured for high-density observability dashboards.
func NewTheme() *Theme {
	th := material.NewTheme()

	// DAEGSA Modern Slate Dark Palette
	bgDark := color.NRGBA{R: 13, G: 17, B: 23, A: 255}        // #0d1117
	bgSurface := color.NRGBA{R: 22, G: 27, B: 34, A: 255}     // #161b22
	bgSurfaceHigh := color.NRGBA{R: 33, G: 38, B: 45, A: 255} // #21262d
	border := color.NRGBA{R: 48, G: 54, B: 61, A: 255}        // #30363d
	borderFocus := color.NRGBA{R: 56, G: 139, B: 253, A: 255} // #388bfd

	textPrimary := color.NRGBA{R: 240, G: 246, B: 252, A: 255}   // #f0f6fc
	textSecondary := color.NRGBA{R: 139, G: 148, B: 158, A: 255} // #8b949e
	textMuted := color.NRGBA{R: 110, G: 118, B: 129, A: 255}     // #6e7681

	primary := color.NRGBA{R: 31, G: 111, B: 235, A: 255}    // #1f6feb
	primaryDark := color.NRGBA{R: 17, G: 88, B: 199, A: 255} // #1158c7
	accent := color.NRGBA{R: 163, G: 113, B: 247, A: 255}    // #a371f7

	success := color.NRGBA{R: 35, G: 134, B: 54, A: 255}     // #238636
	successText := color.NRGBA{R: 63, G: 185, B: 80, A: 255} // #3fb950
	warning := color.NRGBA{R: 187, G: 128, B: 9, A: 255}     // #bb8009
	warningText := color.NRGBA{R: 210, G: 153, B: 34, A: 255} // #d29922
	danger := color.NRGBA{R: 218, G: 54, B: 51, A: 255}      // #da3633
	dangerText := color.NRGBA{R: 248, G: 81, B: 73, A: 255}  // #f85149
	info := color.NRGBA{R: 88, G: 166, B: 255, A: 255}       // #58a6ff

	// Configure Material default colors
	th.Bg = bgDark
	th.Fg = textPrimary
	th.Palette.Bg = bgDark
	th.Palette.Fg = textPrimary
	th.Palette.ContrastBg = primary
	th.Palette.ContrastFg = textPrimary

	return &Theme{
		Material:      th,
		BgDark:        bgDark,
		BgSurface:     bgSurface,
		BgSurfaceHigh: bgSurfaceHigh,
		Border:        border,
		BorderFocus:   borderFocus,
		TextPrimary:   textPrimary,
		TextSecondary: textSecondary,
		TextMuted:     textMuted,
		Primary:       primary,
		PrimaryDark:   primaryDark,
		Accent:        accent,
		Success:       success,
		SuccessText:   successText,
		Warning:       warning,
		WarningText:   warningText,
		Danger:        danger,
		DangerText:    dangerText,
		Info:          info,
		PaddingXS:     unit.Dp(4),
		PaddingSM:     unit.Dp(8),
		PaddingMD:     unit.Dp(12),
		PaddingLG:     unit.Dp(16),
		PaddingXL:     unit.Dp(24),
		RadiusSM:      unit.Dp(4),
		RadiusMD:      unit.Dp(8),
		RadiusLG:      unit.Dp(12),
	}
}
