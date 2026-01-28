package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

type exercise struct {
	name  string
	sets  string
	tempo string
	notes string
}

type block struct {
	name      string
	duration  string
	exercises []exercise
}

type workout struct {
	name   string
	blocks []block
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1)

	blockStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	exerciseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	highlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229"))

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

type state int

const (
	stateMenu state = iota
	stateWorkout
	stateDone
)

type model struct {
	workouts      []workout
	workoutIndex  int
	blockIndex    int
	exerciseIndex int
	state         state
}

func loadWorkouts(filename string) ([]workout, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var raw map[string]map[string][]string
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var workouts []workout
	for name, blocks := range raw {
		w := workout{name: name}
		for blockName, exercises := range blocks {
			b := parseBlock(blockName)
			for _, ex := range exercises {
				b.exercises = append(b.exercises, parseExercise(ex))
			}
			w.blocks = append(w.blocks, b)
		}
		workouts = append(workouts, w)
	}
	return workouts, nil
}

var blockRegex = regexp.MustCompile(`^(.+?)\s*\(([^)]+)\)$`)

func parseBlock(s string) block {
	if matches := blockRegex.FindStringSubmatch(s); matches != nil {
		return block{name: strings.TrimSpace(matches[1]), duration: matches[2]}
	}
	return block{name: s}
}

func parseExercise(s string) exercise {
	parts := strings.Split(s, "|")
	ex := exercise{name: strings.TrimSpace(parts[0])}
	if len(parts) > 1 {
		ex.sets = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		ex.tempo = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 {
		ex.notes = strings.TrimSpace(parts[3])
	}
	return ex
}

func initialModel(workouts []workout) model {
	return model{workouts: workouts, state: stateMenu}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateMenu:
			return m.updateMenu(msg)
		case stateWorkout:
			return m.updateWorkout(msg)
		case stateDone:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			if msg.String() == "m" {
				m.state = stateMenu
				return m, nil
			}
		}
	}
	return m, nil
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.workoutIndex > 0 {
			m.workoutIndex--
		}
	case "down", "j":
		if m.workoutIndex < len(m.workouts)-1 {
			m.workoutIndex++
		}
	case "enter":
		m.state = stateWorkout
		m.blockIndex = 0
		m.exerciseIndex = 0
	}
	return m, nil
}

func (m model) updateWorkout(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "m":
		m.state = stateMenu
		return m, nil
	case "enter", " ", "n":
		return m.nextExercise(), nil
	case "p", "b":
		return m.prevExercise(), nil
	case "s":
		return m.skipBlock(), nil
	}
	return m, nil
}

func (m model) currentWorkout() workout {
	return m.workouts[m.workoutIndex]
}

func (m model) nextExercise() model {
	w := m.currentWorkout()
	block := w.blocks[m.blockIndex]
	if m.exerciseIndex < len(block.exercises)-1 {
		m.exerciseIndex++
	} else if m.blockIndex < len(w.blocks)-1 {
		m.blockIndex++
		m.exerciseIndex = 0
	} else {
		m.state = stateDone
	}
	return m
}

func (m model) prevExercise() model {
	w := m.currentWorkout()
	if m.exerciseIndex > 0 {
		m.exerciseIndex--
	} else if m.blockIndex > 0 {
		m.blockIndex--
		m.exerciseIndex = len(w.blocks[m.blockIndex].exercises) - 1
	}
	return m
}

func (m model) skipBlock() model {
	w := m.currentWorkout()
	if m.blockIndex < len(w.blocks)-1 {
		m.blockIndex++
		m.exerciseIndex = 0
	} else {
		m.state = stateDone
	}
	return m
}

func (m model) View() string {
	switch m.state {
	case stateMenu:
		return m.viewMenu()
	case stateWorkout:
		return m.viewWorkout()
	case stateDone:
		return m.viewDone()
	}
	return ""
}

func (m model) viewMenu() string {
	s := titleStyle.Render("REPCLI") + "\n"
	s += dimStyle.Render("Select a workout") + "\n\n"

	for i, w := range m.workouts {
		if i == m.workoutIndex {
			s += selectedStyle.Render("> "+w.name) + "\n"
		} else {
			s += exerciseStyle.Render("  "+w.name) + "\n"
		}
	}

	s += helpStyle.Render("\n[j/k] navigate • [enter] select • [q] quit")
	return s
}

func (m model) viewWorkout() string {
	w := m.currentWorkout()
	block := w.blocks[m.blockIndex]
	ex := block.exercises[m.exerciseIndex]

	s := titleStyle.Render(strings.ToUpper(w.name)) + "\n"

	if block.duration != "" {
		s += blockStyle.Render(fmt.Sprintf("%s (%s)", block.name, block.duration)) + "\n"
	} else {
		s += blockStyle.Render(block.name) + "\n"
	}

	s += dimStyle.Render(fmt.Sprintf("Block %d/%d • Exercise %d/%d",
		m.blockIndex+1, len(w.blocks),
		m.exerciseIndex+1, len(block.exercises))) + "\n\n"

	s += highlightStyle.Render(ex.name) + "\n"
	if ex.sets != "" {
		s += exerciseStyle.Render("Sets: "+ex.sets) + "\n"
	}
	if ex.tempo != "" {
		s += exerciseStyle.Render("Tempo: "+ex.tempo) + "\n"
	}
	if ex.notes != "" {
		s += dimStyle.Render(ex.notes) + "\n"
	}

	s += helpStyle.Render("\n[enter/n] next • [p] previous • [s] skip block • [m] menu • [q] quit")
	return s
}

func (m model) viewDone() string {
	w := m.currentWorkout()
	return titleStyle.Render(strings.ToUpper(w.name)+" Complete!") + "\n\n" +
		exerciseStyle.Render("Great workout!") + "\n\n" +
		helpStyle.Render("[m] menu • [q] quit")
}

func main() {
	workouts, err := loadWorkouts("workouts.yaml")
	if err != nil {
		fmt.Printf("Error loading workouts: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(workouts))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
