package tui

import "github.com/alderon07/al/internal/presentation"

type themePalette = presentation.Theme

func defaultTheme() themePalette                    { return presentation.DefaultTheme() }
func builtInTheme(name string) themePalette         { return presentation.BuiltInTheme(name) }
func canonicalThemeName(name string) (string, bool) { return presentation.CanonicalThemeName(name) }
func availableThemes() []themePalette               { return presentation.AvailableThemes() }
func completeTheme(theme themePalette) themePalette { return presentation.CompleteTheme(theme) }
func nextTheme(name string) themePalette            { return presentation.NextTheme(name) }

type footerConfig = presentation.FooterConfig
type appearanceConfig = presentation.AppearanceConfig
type pixelIcon = presentation.PixelIcon

func defaultAppearanceConfig() appearanceConfig { return presentation.DefaultAppearanceConfig() }
func defaultFooterConfig() footerConfig         { return presentation.DefaultFooterConfig() }
func validateAppearanceConfig(config appearanceConfig) error {
	return presentation.ValidateAppearanceConfig(config)
}
func validateFooterConfig(config footerConfig) error {
	return presentation.ValidateFooterConfig(config)
}
func renderFooterIcon(value string) (string, error) { return presentation.RenderFooterIcon(value) }
func parsePixelIcon(value string) (pixelIcon, bool) { return presentation.ParsePixelIcon(value) }
func namedPixelIcon(value string) (pixelIcon, bool) { return presentation.NamedPixelIcon(value) }
func renderPixelIcon(icon pixelIcon) string         { return presentation.RenderPixelIcon(icon) }
func configuredPixelIcon(name string) pixelIcon {
	icon, _ := presentation.NamedPixelIcon(name)
	return icon
}

var (
	iconAlias      = configuredPixelIcon("alias")
	iconBrand      = configuredPixelIcon("brand")
	iconCommand    = configuredPixelIcon("command")
	iconContext    = configuredPixelIcon("context")
	iconEdit       = configuredPixelIcon("edit")
	iconFavorite   = configuredPixelIcon("favorite")
	iconFunction   = configuredPixelIcon("function")
	iconHealth     = configuredPixelIcon("health")
	iconHeart      = configuredPixelIcon("heart")
	iconHelp       = configuredPixelIcon("help")
	iconHistory    = configuredPixelIcon("history")
	iconRepository = configuredPixelIcon("repository")
	iconSearch     = configuredPixelIcon("search")
	iconSpark      = configuredPixelIcon("spark")
	iconStats      = configuredPixelIcon("stats")
	iconSync       = configuredPixelIcon("sync")
	iconTheme      = configuredPixelIcon("theme")
)
