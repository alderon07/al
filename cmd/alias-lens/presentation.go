package main

import "github.com/alderon07/al/internal/presentation"

type Theme = presentation.Theme

func defaultTheme() Theme                           { return presentation.DefaultTheme() }
func builtInTheme(name string) Theme                { return presentation.BuiltInTheme(name) }
func canonicalThemeName(name string) (string, bool) { return presentation.CanonicalThemeName(name) }
func availableThemes() []Theme                      { return presentation.AvailableThemes() }
func completeTheme(theme Theme) Theme               { return presentation.CompleteTheme(theme) }
func nextTheme(name string) Theme                   { return presentation.NextTheme(name) }

type FooterConfig = presentation.FooterConfig
type AppearanceConfig = presentation.AppearanceConfig
type pixelIcon = presentation.PixelIcon

func defaultAppearanceConfig() AppearanceConfig { return presentation.DefaultAppearanceConfig() }
func defaultFooterConfig() FooterConfig         { return presentation.DefaultFooterConfig() }
func validateAppearanceConfig(config AppearanceConfig) error {
	return presentation.ValidateAppearanceConfig(config)
}
func validateFooterConfig(config FooterConfig) error {
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
