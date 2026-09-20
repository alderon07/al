// Package tea keeps Alias Lens's small Bubble Tea v1-style model API while
// running it on Bubble Tea v2. Keeping the translation here avoids spreading
// dependency-specific key and view changes through the application.
package tea

import (
	"io"
	"time"
	"unicode"

	tea2 "charm.land/bubbletea/v2"
)

type Msg any
type Cmd func() Msg

type Model interface {
	Init() Cmd
	Update(Msg) (Model, Cmd)
	View() string
}

type KeyType int

const (
	KeyNull      KeyType = 0
	KeyBreak     KeyType = 3
	KeyEnter     KeyType = 13
	KeyBackspace KeyType = 127
	KeyTab       KeyType = 9
	KeyEsc       KeyType = 27
	KeyEscape    KeyType = KeyEsc

	KeyCtrlA KeyType = 1
	KeyCtrlC KeyType = 3
	KeyCtrlD KeyType = 4
	KeyCtrlE KeyType = 5
	KeyCtrlF KeyType = 6
	KeyCtrlG KeyType = 7
	KeyCtrlH KeyType = 8
	KeyCtrlN KeyType = 14
	KeyCtrlR KeyType = 18
	KeyCtrlS KeyType = 19
	KeyCtrlT KeyType = 20
	KeyCtrlU KeyType = 21
	KeyCtrlZ KeyType = 26
)

const (
	KeyRunes KeyType = -(iota + 1)
	KeyUp
	KeyDown
	KeyRight
	KeyLeft
	KeyShiftTab
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDown
	KeyDelete
	KeySpace
	KeyF1
	KeyF2
	KeyF3
)

type KeyMsg struct {
	Type  KeyType
	Runes []rune
	Alt   bool
	Ctrl  bool
	Meta  bool
	Shift bool
	Super bool
	Paste bool
}

func (message KeyMsg) String() string {
	prefix := ""
	if message.Super {
		prefix += "cmd+"
	}
	if message.Meta {
		prefix += "meta+"
	}
	if message.Alt {
		prefix += "alt+"
	}
	if message.Ctrl {
		prefix += "ctrl+"
	}
	if message.Shift {
		prefix += "shift+"
	}
	if message.Type == KeyRunes {
		value := string(message.Runes)
		if message.Paste {
			value = "[" + value + "]"
		}
		return prefix + value
	}
	names := map[KeyType]string{
		KeyEnter: "enter", KeyBackspace: "backspace", KeyTab: "tab", KeyEsc: "esc",
		KeySpace: " ", KeyUp: "up", KeyDown: "down", KeyRight: "right", KeyLeft: "left",
		KeyShiftTab: "shift+tab", KeyHome: "home", KeyEnd: "end", KeyPgUp: "pgup",
		KeyPgDown: "pgdown", KeyDelete: "delete", KeyF1: "f1", KeyF2: "f2", KeyF3: "f3",
		KeyCtrlA: "ctrl+a", KeyCtrlC: "ctrl+c", KeyCtrlD: "ctrl+d", KeyCtrlE: "ctrl+e",
		KeyCtrlF: "ctrl+f", KeyCtrlG: "ctrl+g", KeyCtrlH: "ctrl+h", KeyCtrlN: "ctrl+n", KeyCtrlR: "ctrl+r",
		KeyCtrlS: "ctrl+s", KeyCtrlT: "ctrl+t", KeyCtrlU: "ctrl+u", KeyCtrlZ: "ctrl+z",
	}
	return prefix + names[message.Type]
}

type WindowSizeMsg struct {
	Width  int
	Height int
}

type FocusMsg struct{}
type BlurMsg struct{}

type programConfig struct {
	input       io.Reader
	output      io.Writer
	inputSet    bool
	outputSet   bool
	altScreen   bool
	reportFocus bool
}

type ProgramOption func(*programConfig)

func WithAltScreen() ProgramOption {
	return func(config *programConfig) { config.altScreen = true }
}

func WithReportFocus() ProgramOption {
	return func(config *programConfig) { config.reportFocus = true }
}

func WithInput(input io.Reader) ProgramOption {
	return func(config *programConfig) {
		config.input = input
		config.inputSet = true
	}
}

func WithOutput(output io.Writer) ProgramOption {
	return func(config *programConfig) {
		config.output = output
		config.outputSet = true
	}
}

type Program struct {
	model  Model
	config programConfig
}

func NewProgram(model Model, options ...ProgramOption) *Program {
	program := &Program{model: model}
	for _, option := range options {
		option(&program.config)
	}
	return program
}

func (program *Program) Run() (Model, error) {
	wrapped := &v2Model{model: program.model, config: program.config}
	options := make([]tea2.ProgramOption, 0, 2)
	if program.config.inputSet {
		options = append(options, tea2.WithInput(program.config.input))
	}
	if program.config.outputSet {
		options = append(options, tea2.WithOutput(program.config.output))
	}
	result, err := tea2.NewProgram(wrapped, options...).Run()
	if current, ok := result.(*v2Model); ok {
		return current.model, err
	}
	return wrapped.model, err
}

type v2Model struct {
	model  Model
	config programConfig
}

func (model *v2Model) Init() tea2.Cmd {
	return adaptCommand(model.model.Init())
}

func (model *v2Model) Update(message tea2.Msg) (tea2.Model, tea2.Cmd) {
	next, command := model.model.Update(translateMessage(message))
	model.model = next
	return model, adaptCommand(command)
}

func (model *v2Model) View() tea2.View {
	view := tea2.NewView(model.model.View())
	view.AltScreen = model.config.altScreen
	view.ReportFocus = model.config.reportFocus
	return view
}

func adaptCommand(command Cmd) tea2.Cmd {
	if command == nil {
		return nil
	}
	return func() tea2.Msg { return command() }
}

func translateMessage(message tea2.Msg) Msg {
	switch value := message.(type) {
	case tea2.KeyPressMsg:
		return translateKey(value.Key())
	case tea2.WindowSizeMsg:
		return WindowSizeMsg{Width: value.Width, Height: value.Height}
	case tea2.FocusMsg:
		return FocusMsg{}
	case tea2.BlurMsg:
		return BlurMsg{}
	case tea2.PasteMsg:
		return KeyMsg{Type: KeyRunes, Runes: []rune(value.Content), Paste: true}
	default:
		return message
	}
}

func translateKey(key tea2.Key) KeyMsg {
	alt := key.Mod&tea2.ModAlt != 0
	ctrl := key.Mod&tea2.ModCtrl != 0
	meta := key.Mod&tea2.ModMeta != 0
	shift := key.Mod&tea2.ModShift != 0
	super := key.Mod&tea2.ModSuper != 0
	if ctrl {
		if keyType, ok := controlKeyType(key.Code); ok {
			return KeyMsg{Type: keyType, Alt: alt, Meta: meta, Shift: shift, Super: super}
		}
	}
	special := map[rune]KeyType{
		tea2.KeyEnter: KeyEnter, tea2.KeyBackspace: KeyBackspace,
		tea2.KeyEscape: KeyEsc, tea2.KeySpace: KeySpace, tea2.KeyUp: KeyUp, tea2.KeyDown: KeyDown,
		tea2.KeyRight: KeyRight, tea2.KeyLeft: KeyLeft, tea2.KeyHome: KeyHome, tea2.KeyEnd: KeyEnd,
		tea2.KeyPgUp: KeyPgUp, tea2.KeyPgDown: KeyPgDown, tea2.KeyDelete: KeyDelete,
		tea2.KeyF1: KeyF1, tea2.KeyF2: KeyF2, tea2.KeyF3: KeyF3,
	}
	if key.Code == tea2.KeyTab {
		if key.Mod&tea2.ModShift != 0 {
			return KeyMsg{Type: KeyShiftTab, Alt: alt, Ctrl: ctrl, Meta: meta, Super: super}
		}
		return KeyMsg{Type: KeyTab, Alt: alt, Ctrl: ctrl, Meta: meta, Super: super}
	}
	if keyType, ok := special[key.Code]; ok {
		return KeyMsg{Type: keyType, Alt: alt, Ctrl: ctrl, Meta: meta, Shift: shift, Super: super}
	}
	text := key.Text
	if text == "" && key.Code >= 0 {
		text = string(key.Code)
	}
	return KeyMsg{Type: KeyRunes, Runes: []rune(text), Alt: alt, Ctrl: ctrl, Meta: meta, Shift: shift, Super: super}
}

func controlKeyType(code rune) (KeyType, bool) {
	keys := map[rune]KeyType{
		'a': KeyCtrlA, 'c': KeyCtrlC, 'd': KeyCtrlD, 'e': KeyCtrlE, 'f': KeyCtrlF,
		'g': KeyCtrlG, 'h': KeyCtrlH, 'n': KeyCtrlN, 'r': KeyCtrlR, 's': KeyCtrlS, 't': KeyCtrlT, 'u': KeyCtrlU,
		'z': KeyCtrlZ,
	}
	keyType, ok := keys[unicode.ToLower(code)]
	return keyType, ok
}

func Quit() Msg {
	return tea2.Quit()
}

func Tick(duration time.Duration, callback func(time.Time) Msg) Cmd {
	command := tea2.Tick(duration, func(now time.Time) tea2.Msg { return callback(now) })
	return func() Msg { return command() }
}
