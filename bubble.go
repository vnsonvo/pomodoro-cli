package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	padding      = 2
	maxWidth     = 80
	workSession  = "Work"
	breakSession = "Break"
)

type tickMsg time.Time

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render

var keys = keyMap{
	Stop: key.NewBinding(
		key.WithKeys("x"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
	),
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Command..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 20

	ta.SetWidth(30)
	ta.SetHeight(2)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	ta.ShowLineNumbers = false

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{sessions: loadSessions(), keys: keys,
		progress: progress.New(progress.WithDefaultGradient()), textarea: ta,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.textarea, _ = m.textarea.Update(msg)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inSession {
			if m.opening || m.closing {
				return m, nil
			}
			switch {
			case key.Matches(msg, m.keys.Stop):

				if m.inSession {
					m.inSession = false
					m.textarea.Reset()
				}
				return m, nil
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			}
		}

		if m.showSession && key.Matches(msg, m.keys.Stop) {
			m.showSession = false
			m.textarea.Reset()
			return m, nil
		}

		m.err = ""
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyType('q'):
			return m, nil
		case tea.KeyEnter:
			command := m.textarea.Value()
			m.textarea.Reset()

			switch {
			case command == "q":
				return m, tea.Quit
			case strings.HasPrefix(command, "s"):
				numOfMinutes, ok := checkValidMinute(&m, command)
				if !ok {
					return m, nil
				}

				if numOfMinutes == 0 {
					numOfMinutes = 25
				}

				if !m.inSession {
					m.startTime = time.Now()
					m.sessionType = workSession
					m.timerDuration = time.Duration(numOfMinutes) * time.Minute
					m.remainingTime = m.timerDuration + 3*time.Second
					m.percent = 0
					m.inSession = true
					m.opening = true
					m.closing = false
					return m, tickCmd()
				} else {
					return m, nil
				}
			case strings.HasPrefix(command, "b"):
				numOfMinutes, ok := checkValidMinute(&m, command)
				if !ok {
					return m, nil
				}

				if numOfMinutes == 0 {
					numOfMinutes = 5
				}

				if !m.inSession {
					m.startTime = time.Now()
					m.sessionType = breakSession
					m.timerDuration = time.Duration(numOfMinutes) * time.Minute
					m.remainingTime = m.timerDuration + 3*time.Second
					m.percent = 0
					m.inSession = true
					m.opening = true
					m.closing = false
					return m, tickCmd()
				} else {
					return m, nil
				}
			case strings.HasPrefix(command, "l"):
				if command == "l" {
					m.printDifferentDate = false
					m.showSession = true
				} else {
					spacing := command[1:]
					if !strings.HasPrefix(spacing, " ") {
						m.err = "Invalid command"
						return m, nil
					}

					dateStr := strings.TrimSpace(command[2:])

					date, err := time.Parse(time.DateOnly, dateStr)
					if err != nil {
						m.err = "Invalid date format"
						return m, nil
					}
					m.printDifferentDate = true
					m.datePrint = date
					m.showSession = true
				}

				return m, nil

			default:
				if !m.inSession {
					showHelper()
				}
				m.err = "Invalid command"
				return m, nil
			}
		default:
			if !m.inSession {
				showHelper()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case tickMsg:
		if !m.inSession {
			return m, nil
		}

		m.remainingTime -= 1 * time.Second

		if m.opening {
			if m.remainingTime.Milliseconds() <= m.timerDuration.Milliseconds() {
				m.opening = false
			}

			return m, tickCmd()
		}

		if m.remainingTime.Seconds() <= -4 {
			m.closing = false
			m.inSession = false

			if m.sessionType == workSession {
				endSession := time.Now()
				m.sessions = append(m.sessions, session{StartTime: m.startTime,
					EndTime: endSession, Duration: m.timerDuration})
				err := saveSessions(m.sessions)
				if err != nil {
					m.err = err.Error()
				}
			}

			return m, nil
		}

		if m.remainingTime.Seconds() <= 0 {
			m.closing = true
			return m, tickCmd()
		}

		m.percent = 1 - float64(m.remainingTime.Milliseconds())/float64(m.timerDuration.Milliseconds())

		return m, tickCmd()

	default:
		return m, nil
	}
}

func (m model) View() string {
	if m.showSession {
		return fmt.Sprintf("\n%s\n%s",
			printSessions(m.sessions, m.printDifferentDate, m.datePrint),
			helpStyle(" - Press 'x' to stop\n"))
	}

	if !m.inSession {
		return fmt.Sprintf(
			"\n%s\n%s\n\n%s\n\n",
			showHelper(),
			m.textarea.View(),
			m.err,
		)
	}
	if m.opening {
		return fmt.Sprintf("Ready to start new %s session for %.0f minutes in %d seconds...",
			m.sessionType,
			m.timerDuration.Minutes(),
			int(m.remainingTime.Seconds()-m.timerDuration.Seconds()))
	}

	if m.closing {
		if m.sessionType == workSession {
			return fmt.Sprintf("You have completed one %s session. Keep it up 💪",
				m.sessionType)
		}
		return fmt.Sprintf("Regained your energy with short %s. Let's start %s session.",
			breakSession,
			workSession)
	}

	return fmt.Sprintf("\n%s Timer: %s left\n\n  %v\n\n\n%v\n",
		m.sessionType,
		m.remainingTime,
		m.progress.ViewAs(m.percent),
		helpStyle(" - Press 'x' to stop\n - Press 'q' to quit"))
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}
