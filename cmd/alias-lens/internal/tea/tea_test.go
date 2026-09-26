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
		{name: "context mark", key: tea2.Key{Code: 'b', Mod: tea2.ModCtrl}, want: KeyMsg{Type: KeyCtrlB}},
		{name: "unknown ctrl", key: tea2.Key{Code: 'x', Mod: tea2.ModCtrl}, want: KeyMsg{Type: KeyRunes, Runes: []rune{'x'}, Ctrl: true}},
		{name: "control punctuation", key: tea2.Key{Code: ',', Mod: tea2.ModCtrl}, want: KeyMsg{Type: KeyRunes, Runes: []rune{','}, Ctrl: true}},
		{name: "control shifted punctuation", key: tea2.Key{Code: '/', ShiftedCode: '?', Mod: tea2.ModCtrl | tea2.ModShift}, want: KeyMsg{Type: KeyRunes, Runes: []rune{'?'}, Ctrl: true, Shift: true}},
		{name: "command", key: tea2.Key{Code: 'n', Mod: tea2.ModSuper}, want: KeyMsg{Type: KeyRunes, Runes: []rune{'n'}, Super: true}},
		{name: "meta is not command", key: tea2.Key{Code: 'n', Mod: tea2.ModMeta}, want: KeyMsg{Type: KeyRunes, Runes: []rune{'n'}, Meta: true}},
		{name: "command shift", key: tea2.Key{Code: 's', Mod: tea2.ModSuper | tea2.ModShift}, want: KeyMsg{Type: KeyRunes, Runes: []rune{'s'}, Shift: true, Super: true}},
		{name: "shift tab", key: tea2.Key{Code: tea2.KeyTab, Mod: tea2.ModShift}, want: KeyMsg{Type: KeyShiftTab}},
		{name: "function", key: tea2.Key{Code: tea2.KeyF2}, want: KeyMsg{Type: KeyF2}},
		{name: "settings function", key: tea2.Key{Code: tea2.KeyF3}, want: KeyMsg{Type: KeyF3}},
		{name: "themes function", key: tea2.Key{Code: tea2.KeyF4}, want: KeyMsg{Type: KeyF4}},
		{name: "refresh function", key: tea2.Key{Code: tea2.KeyF5}, want: KeyMsg{Type: KeyF5}},
		{name: "sync function", key: tea2.Key{Code: tea2.KeyF6}, want: KeyMsg{Type: KeyF6}},
		{name: "health function", key: tea2.Key{Code: tea2.KeyF7}, want: KeyMsg{Type: KeyF7}},
		{name: "revisions function", key: tea2.Key{Code: tea2.KeyF8}, want: KeyMsg{Type: KeyF8}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := translateKey(test.key)
			if got.Type != test.want.Type || string(got.Runes) != string(test.want.Runes) || got.Alt != test.want.Alt || got.Ctrl != test.want.Ctrl || got.Meta != test.want.Meta || got.Shift != test.want.Shift || got.Super != test.want.Super {
				t.Fatalf("translateKey(%+v) = %+v, want %+v", test.key, got, test.want)
			}
		})
	}
}

func TestProgramOptionsReachV2View(t *testing.T) {
	model := &testModel{background: "#112233"}
	program := NewProgram(model, WithAltScreen(), WithReportFocus())
	wrapped := &v2Model{model: program.model, config: program.config}
	view := wrapped.View()
	if !view.AltScreen || !view.ReportFocus || view.Content != "test" {
		t.Fatalf("v2 view options were not preserved: %+v", view)
	}
	red, green, blue, alpha := view.BackgroundColor.RGBA()
	if red != 0x1111 || green != 0x2222 || blue != 0x3333 || alpha != 0xffff {
		t.Fatalf("v2 view background = %#v", view.BackgroundColor)
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

type testModel struct {
	background string
}

func (*testModel) Init() Cmd                     { return nil }
func (model *testModel) Update(Msg) (Model, Cmd) { return model, nil }
func (*testModel) View() string                  { return "test" }
func (model *testModel) TerminalBackground() string {
	return model.background
}
