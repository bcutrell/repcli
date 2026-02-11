package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	group  string
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

	timerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("46"))

	timerExpiredStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("196"))
)

type historyEntry struct {
	Workout string `json:"workout"`
	Date    string `json:"date"`
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type state int

const (
	stateMenu state = iota
	stateWorkout
	stateDone
	stateHistory
)

type model struct {
	workouts      []workout
	workoutIndex  int
	blockIndex    int
	exerciseIndex int
	state         state

	timerRunning bool
	timerLeft    time.Duration
	timerTotal   time.Duration

	history      []historyEntry
	historyMonth time.Month
	historyYear  int
}

func loadWorkouts(filename string) ([]workout, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var groups []struct {
		Group    string `yaml:"group"`
		Workouts []struct {
			Name   string `yaml:"name"`
			Blocks []struct {
				Name      string   `yaml:"name"`
				Exercises []string `yaml:"exercises"`
			} `yaml:"blocks"`
		} `yaml:"workouts"`
	}
	if err := yaml.Unmarshal(data, &groups); err != nil {
		return nil, err
	}

	var workouts []workout
	for _, g := range groups {
		for _, yw := range g.Workouts {
			w := workout{name: yw.Name, group: g.Group}
			for _, yb := range yw.Blocks {
				b := parseBlock(yb.Name)
				for _, ex := range yb.Exercises {
					b.exercises = append(b.exercises, parseExercise(ex))
				}
				w.blocks = append(w.blocks, b)
			}
			workouts = append(workouts, w)
		}
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

var timedBlockRegex = regexp.MustCompile(`(?i)(AMRAP|E2MOM|EMOM)`)
var timerMinRegex = regexp.MustCompile(`(\d+)\s*-?\s*min`)

func blockTimerMinutes(b block) int {
	if !timedBlockRegex.MatchString(b.name) && !timedBlockRegex.MatchString(b.duration) {
		return 0
	}
	if m := timerMinRegex.FindStringSubmatch(b.name); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if m := timerMinRegex.FindStringSubmatch(b.duration); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func (m *model) resetBlockTimer() {
	w := m.currentWorkout()
	b := w.blocks[m.blockIndex]
	mins := blockTimerMinutes(b)
	if mins > 0 {
		m.timerTotal = time.Duration(mins) * time.Minute
		m.timerLeft = m.timerTotal
	} else {
		m.timerTotal = 0
		m.timerLeft = 0
	}
	m.timerRunning = false
}

const historyFile = "history.json"

func loadHistory() []historyEntry {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return nil
	}
	var entries []historyEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

func saveHistory(entries []historyEntry) {
	data, _ := json.MarshalIndent(entries, "", "  ")
	os.WriteFile(historyFile, data, 0644)
}

func initialModel(workouts []workout) model {
	now := time.Now()
	return model{
		workouts:     workouts,
		state:        stateMenu,
		history:      loadHistory(),
		historyMonth: now.Month(),
		historyYear:  now.Year(),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if !m.timerRunning || m.timerLeft <= 0 {
			return m, nil
		}
		m.timerLeft -= time.Second
		if m.timerLeft <= 0 {
			m.timerLeft = 0
			m.timerRunning = false
			return m, nil
		}
		return m, tickCmd()
	case tea.KeyMsg:
		switch m.state {
		case stateMenu:
			return m.updateMenu(msg)
		case stateWorkout:
			return m.updateWorkout(msg)
		case stateHistory:
			return m.updateHistory(msg)
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
		m.resetBlockTimer()
	case "h":
		now := time.Now()
		m.state = stateHistory
		m.historyMonth = now.Month()
		m.historyYear = now.Year()
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
		m = m.nextExercise()
		if m.state == stateWorkout {
			return m, nil
		}
		return m, nil
	case "p", "b":
		oldBlock := m.blockIndex
		m = m.prevExercise()
		if m.blockIndex != oldBlock {
			m.resetBlockTimer()
		}
		return m, nil
	case "s":
		m = m.skipBlock()
		if m.state == stateWorkout {
			m.resetBlockTimer()
		}
		return m, nil
	case "t":
		if m.timerTotal <= 0 {
			return m, nil
		}
		if m.timerLeft <= 0 {
			m.timerLeft = m.timerTotal
			m.timerRunning = true
			return m, tickCmd()
		}
		m.timerRunning = !m.timerRunning
		if m.timerRunning {
			return m, tickCmd()
		}
		return m, nil
	}
	return m, nil
}

func (m model) currentWorkout() workout {
	return m.workouts[m.workoutIndex]
}

func (m model) nextExercise() model {
	w := m.currentWorkout()
	b := w.blocks[m.blockIndex]
	oldBlock := m.blockIndex
	if m.exerciseIndex < len(b.exercises)-1 {
		m.exerciseIndex++
	} else if m.blockIndex < len(w.blocks)-1 {
		m.blockIndex++
		m.exerciseIndex = 0
	} else {
		m.state = stateDone
		m.history = append(m.history, historyEntry{
			Workout: w.name,
			Date:    time.Now().Format("2006-01-02"),
		})
		saveHistory(m.history)
		return m
	}
	if m.blockIndex != oldBlock {
		m.resetBlockTimer()
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
	case stateHistory:
		return m.viewHistory()
	}
	return ""
}

func (m model) viewMenu() string {
	s := titleStyle.Render("REPCLI") + "\n"
	s += dimStyle.Render("Select a workout") + "\n\n"

	currentGroup := ""
	for i, w := range m.workouts {
		if w.group != currentGroup {
			if currentGroup != "" {
				s += "\n"
			}
			s += blockStyle.Render(w.group) + "\n"
			currentGroup = w.group
		}
		if i == m.workoutIndex {
			s += selectedStyle.Render("  > "+w.name) + "\n"
		} else {
			s += exerciseStyle.Render("    "+w.name) + "\n"
		}
	}

	s += helpStyle.Render("\n[j/k] navigate • [enter] select • [h] history • [q] quit")
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

	if m.timerTotal > 0 {
		mins := int(m.timerLeft.Minutes())
		secs := int(m.timerLeft.Seconds()) % 60
		ts := fmt.Sprintf("%02d:%02d", mins, secs)
		if m.timerLeft <= 0 {
			s += timerExpiredStyle.Render("TIME!") + "\n"
		} else if m.timerRunning {
			s += timerStyle.Render(ts) + "\n"
		} else {
			s += dimStyle.Render(ts+" [paused]") + "\n"
		}
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

	help := "[enter/n] next • [p] previous • [s] skip block"
	if m.timerTotal > 0 {
		help += " • [t] timer"
	}
	help += " • [m] menu • [q] quit"
	s += helpStyle.Render("\n" + help)
	return s
}

func (m model) viewDone() string {
	w := m.currentWorkout()
	return titleStyle.Render(strings.ToUpper(w.name)+" Complete!") + "\n\n" +
		exerciseStyle.Render("Great workout!") + "\n\n" +
		helpStyle.Render("[m] menu • [q] quit")
}

func (m model) updateHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "m":
		m.state = stateMenu
		return m, nil
	case "left", "h":
		m.historyMonth--
		if m.historyMonth < 1 {
			m.historyMonth = 12
			m.historyYear--
		}
	case "right", "l":
		m.historyMonth++
		if m.historyMonth > 12 {
			m.historyMonth = 1
			m.historyYear++
		}
	}
	return m, nil
}

func (m model) viewHistory() string {
	s := titleStyle.Render("HISTORY") + "\n\n"
	s += renderCalendar(m.historyYear, m.historyMonth, m.history)
	s += helpStyle.Render("\n[h/l] month • [m] menu • [q] quit")
	return s
}

func renderCalendar(year int, month time.Month, entries []historyEntry) string {
	workoutDays := map[int]bool{}
	var monthEntries []historyEntry
	for _, e := range entries {
		t, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			continue
		}
		if t.Year() == year && t.Month() == month {
			workoutDays[t.Day()] = true
			monthEntries = append(monthEntries, e)
		}
	}

	s := fmt.Sprintf("  %s %d\n", month.String(), year)
	s += "  Mon  Tue  Wed  Thu  Fri  Sat  Sun\n"

	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	dow := int(first.Weekday())
	if dow == 0 {
		dow = 7
	}
	s += strings.Repeat("     ", dow-1)

	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
	for d := 1; d <= daysInMonth; d++ {
		if workoutDays[d] {
			s += selectedStyle.Render(fmt.Sprintf(" *%2d", d)) + " "
		} else {
			s += fmt.Sprintf("  %2d ", d)
		}
		wd := (dow - 1 + d) % 7
		if wd == 0 {
			s += "\n"
		}
	}
	s += "\n"

	if len(monthEntries) > 0 {
		s += "\n"
		for _, e := range monthEntries {
			t, _ := time.Parse("2006-01-02", e.Date)
			s += selectedStyle.Render(fmt.Sprintf("  * %s (%s %d)", e.Workout, t.Month().String()[:3], t.Day())) + "\n"
		}
	}

	return s
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
