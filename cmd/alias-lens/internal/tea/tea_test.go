package tea

import (
	"testing"

	tea2 "charm.land/bubbletea/v2"
)

func TestTranslateV2Keys(t *testing.T) {
	tests := []struct {
		name string
		key  tea2.Key
		want KeyMsg
	}{
		{name: "rune", key: tea2.Key{Code: 'a', Text: "a"}, want: KeyMsg{Type: KeyRunes, Runes: []rune{'a'}}},
		{name: "ctrl", key: tea2.Key{Code: 'g', Mod: tea2.ModCtrl}, want: KeyMsg{Type: KeyCtrlG}},
		{name: "shift tab", key: tea2.Key{Code: tea2.KeyTab, Mod: tea2.ModShift}, want: KeyMsg{Type: KeyShiftTab}},
		{name: "function", key: tea2.Key{Code: tea2.KeyF2}, want: KeyMsg{Type: KeyF2}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := translateKey(test.key)
			if got.Type != test.want.Type || string(got.Runes) != string(test.want.Runes) || got.Alt != test.want.Alt {
				t.Fatalf("translateKey(%+v) = %+v, want %+v", test.key, got, test.want)
			}
		})
	}
}

func TestProgramOptionsReachV2View(t *testing.T) {
	model := &testModel{}
	program := NewProgram(model, WithAltScreen(), WithReportFocus())
	wrapped := &v2Model{model: program.model, config: program.config}
	view := wrapped.View()
	if !view.AltScreen || !view.ReportFocus || view.Content != "test" {
		t.Fatalf("v2 view options were not preserved: %+v", view)
	}
}

func TestTranslateV2Paste(t *testing.T) {
	message, ok := translateMessage(tea2.PasteMsg{Content: "git status"}).(KeyMsg)
	if !ok || message.Type != KeyRunes || string(message.Runes) != "git status" || !message.Paste {
		t.Fatalf("paste translation = %#v", message)
	}
}

func TestQuitCommandUsesV2QuitMessage(t *testing.T) {
	if _, ok := adaptCommand(Quit)().(tea2.QuitMsg); !ok {
		t.Fatal("quit command did not produce a Bubble Tea v2 quit message")
	}
}

type testModel struct{}

func (*testModel) Init() Cmd                     { return nil }
func (model *testModel) Update(Msg) (Model, Cmd) { return model, nil }
func (*testModel) View() string                  { return "test" }
