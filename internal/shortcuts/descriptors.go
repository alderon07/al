package shortcuts

import "encoding/json"

type Descriptor struct {
	Action      Action
	Name, Scope string
}

func Descriptors() []Descriptor {
	result := make([]Descriptor, 0, len(shortcutDefinitions))
	for _, definition := range shortcutDefinitions {
		result = append(result, Descriptor{definition.action, ActionName(definition.action), definition.scope})
	}
	return result
}
func (profile Profile) WithOverrides(overrides map[string]string) Profile {
	profile.overrides = ""
	if len(overrides) != 0 {
		encoded, _ := json.Marshal(overrides)
		profile.overrides = string(encoded)
	}
	return profile
}

func Labels(profile Profile, action Action) []string {
	for _, definition := range shortcutDefinitions {
		if definition.action == action {
			choice := shortcutChoiceForProfile(definition, profile)
			result := make([]string, 0, len(choice.bindings))
			for _, binding := range choice.bindings {
				result = append(result, binding.label)
			}
			return result
		}
	}
	return nil
}
