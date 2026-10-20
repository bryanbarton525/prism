package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bryanbarton525/prism/internal/llm"
	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

var errModelCancelled = errors.New("model selection cancelled; configuration not saved")

// The picker state machine is independent of terminal ownership, so navigation,
// text entry, and cancellation can be tested without a real terminal.
type modelPicker struct {
	read   func() (rune, error)
	out    io.Writer
	width  int
	height int
}

func (p modelPicker) line(text string) string {
	text = terminalLabel(text)
	if p.width <= 0 {
		return text
	}
	var clipped []rune
	columns := 0
	for _, r := range text {
		w := 1
		if r > 127 {
			w = 2
		}
		if columns+w > p.width-2 {
			return string(clipped) + "…"
		}
		clipped = append(clipped, r)
		columns += w
	}
	return text
}

func (p modelPicker) key() (rune, error) {
	r, err := p.read()
	if err != nil {
		return 0, err
	}
	if r == 3 || r == 4 {
		return 0, errModelCancelled
	}
	if r == 27 {
		next, err := p.read()
		if err != nil {
			return 0, err
		}
		if next != '[' && next != 'O' {
			return 0, nil
		}
		next, err = p.read()
		if err != nil {
			return 0, err
		}
		switch next {
		case 'A':
			return '↑', nil
		case 'B':
			return '↓', nil
		}
		return 0, nil
	}
	return r, nil
}

func (p modelPicker) choose(label string, choices []string, selected int) (int, error) {
	if len(choices) == 0 {
		return 0, fmt.Errorf("no choices available")
	}
	if selected < 0 || selected >= len(choices) {
		selected = 0
	}
	rows := 0
	for {
		if rows > 0 {
			fmt.Fprintf(p.out, "\x1b[%dA\r\x1b[J", rows)
		}
		fmt.Fprintf(p.out, "%s\r\n", p.line(label+" (↑/↓, Enter; Ctrl-C cancels)"))
		start := max(0, selected-3)
		pageSize := 8
		if p.height > 0 {
			pageSize = max(1, min(pageSize, p.height-3))
		}
		if selected >= start+pageSize {
			start = selected - pageSize + 1
		}
		end := min(len(choices), start+pageSize)
		rows = 1
		for i := start; i < end; i++ {
			marker := "  "
			if i == selected {
				marker = "> "
			}
			fmt.Fprintf(p.out, "%s\r\n", p.line(marker+choices[i]))
			rows++
		}
		r, err := p.key()
		if err != nil {
			return 0, err
		}
		switch r {
		case '↑', 'k':
			selected = (selected + len(choices) - 1) % len(choices)
		case '↓', 'j':
			selected = (selected + 1) % len(choices)
		case '\r', '\n':
			return selected, nil
		}
	}
}

func (p modelPicker) text(label, initial string) (string, error) {
	var value []rune
	for {
		prompt := label
		if initial != "" {
			prompt += " [" + initial + "]"
		}
		fmt.Fprintf(p.out, "\r\x1b[2K%s", p.line(prompt+": "+string(value)))
		r, err := p.key()
		if err != nil {
			return "", err
		}
		switch r {
		case '\r', '\n':
			fmt.Fprint(p.out, "\r\n")
			if len(value) == 0 {
				return initial, nil
			}
			return strings.TrimSpace(string(value)), nil
		case 127, 8:
			if len(value) > 0 {
				value = value[:len(value)-1]
			}
		default:
			if r >= 32 && r != '↑' && r != '↓' {
				value = append(value, r)
			}
		}
	}
}

func terminalLabel(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, s)
}

func pickModelTarget(cmd *cobra.Command, flags modelSetFlags) (runtime.Config, string, error) {
	in, inOK := cmd.InOrStdin().(*os.File)
	out, outOK := cmd.OutOrStdout().(*os.File)
	if !inOK || !outOK || !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return runtime.Config{}, "", fmt.Errorf("model picker requires an interactive terminal; use prism model set <model> --engine ollama (or add --skip-check for offline setup)")
	}
	state, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return runtime.Config{}, "", err
	}
	defer term.Restore(int(in.Fd()), state)
	defer fmt.Fprint(out, "\r\n")
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer cancel()
	type inputEvent struct {
		key rune
		err error
	}
	events := make(chan inputEvent)
	go func() {
		reader := bufio.NewReader(in)
		for {
			key, _, err := reader.ReadRune()
			if key == 3 || key == 4 {
				cancel()
				return
			}
			select {
			case events <- inputEvent{key, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	width, height, _ := term.GetSize(int(out.Fd()))
	picker := modelPicker{out: out, width: width, height: height, read: func() (rune, error) {
		select {
		case event := <-events:
			return event.key, event.err
		case <-ctx.Done():
			return 0, errModelCancelled
		}
	}}
	target, keyEnv, err := picker.selectTarget(ctx, cmd, flags)
	if ctx.Err() != nil {
		return runtime.Config{}, "", errModelCancelled
	}
	return target, keyEnv, err
}

func (p modelPicker) selectTarget(ctx context.Context, cmd *cobra.Command, flags modelSetFlags) (runtime.Config, string, error) {
	current := effectivePrimary()
	engines := []string{"ollama", "sglang", "vllm"}
	engine := string(current.Engine)
	if flags.engine != "" {
		engine = flags.engine
	}
	selected := 0
	for i, choice := range engines {
		if choice == engine {
			selected = i
		}
	}
	if flags.engine == "" {
		index, err := p.choose("Serving engine", engines, selected)
		if err != nil {
			return runtime.Config{}, "", err
		}
		engine = engines[index]
	}
	flags.engine = engine
	endpoint := flags.url
	if endpoint == "" {
		if engine == string(current.Engine) {
			endpoint = current.BaseURL
		} else if engine == "ollama" {
			endpoint = gf.ollamaHost
		}
		var err error
		endpoint, err = p.text("Endpoint URL", endpoint)
		if err != nil {
			return runtime.Config{}, "", err
		}
	}
	flags.url = endpoint
	// Select a target first so no credential is sent to a newly chosen endpoint.
	target, keyEnv, err := selectModelTarget(cmd, "picker-placeholder", flags)
	if err != nil {
		return target, keyEnv, err
	}
	choices := []string{}
	if !flags.skipCheck {
		models, err := llm.ListModels(ctx, target)
		if err != nil {
			return target, keyEnv, fmt.Errorf("configuration not saved: %w; retry after fixing the endpoint/authentication or use --skip-check", err)
		}
		choices = append(choices, models...)
	}
	choices = append(choices, "Other — type a model ID")
	selected = 0
	for i, model := range choices[:len(choices)-1] {
		if model == current.Model {
			selected = i
		}
	}
	index, err := p.choose("Offload model", choices, selected)
	if err != nil {
		return target, keyEnv, err
	}
	model := choices[index]
	if index == len(choices)-1 {
		model, err = p.text("Model ID", current.Model)
		if err != nil {
			return target, keyEnv, err
		}
	}
	target, keyEnv, err = selectModelTarget(cmd, model, flags)
	if err != nil {
		return target, keyEnv, err
	}
	action := "Save configuration"
	if flags.dryRun {
		action = "Preview configuration"
	}
	fmt.Fprintf(p.out, "%s\r\n%s\r\n%s\r\n", p.line("Engine: "+string(target.Engine)), p.line("Endpoint: "+target.BaseURL), p.line("Model: "+target.Model))
	index, err = p.choose("Apply shared runtime selection?", []string{action, "Cancel"}, 0)
	if err != nil {
		return target, keyEnv, err
	}
	if index != 0 {
		return target, keyEnv, errModelCancelled
	}
	return target, keyEnv, nil
}
