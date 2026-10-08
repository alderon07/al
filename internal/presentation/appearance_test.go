package presentation

import (
	"strings"
	"testing"
)

func TestFooterConfigurationValidation(t *testing.T) {
	tests := []FooterConfig{
		{Message: "bad\nmessage", Icon: "heart"},
		{Message: strings.Repeat("x", 81), Icon: "heart"},
		{Message: "two {icon} tokens {icon}", Icon: "heart"},
		{Message: "bad {icon}", Icon: "###/...."},
		{Message: "bad {icon}", Icon: "####/..x."},
		{Message: "bad {icon}", Icon: "emoji:"},
		{Message: "bad {icon}", Icon: "emoji:🚀🚀"},
		{Message: "bad tone", Icon: "none", Tone: "loud"},
		{Message: "bad rule", Icon: "none", Rule: "double"},
	}
	for _, config := range tests {
		if err := ValidateFooterConfig(config); err == nil {
			t.Errorf("invalid footer config was accepted: %#v", config)
		}
	}
}
