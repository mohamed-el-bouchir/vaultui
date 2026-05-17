package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateInitVaultRoot state = iota
	stateNotebookList
	stateAuth
	stateNewNotebookPassword
	stateNewNotebookPasswordConfirm
	stateNoteList
	stateNewNoteName
	stateSettings
	stateSettingVaultRoot
	stateDeleteAuth
	stateChangePasswordAuth
	stateChangePasswordNew
	stateChangePasswordConfirm
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	state          state
	config         *Config
	list           list.Model
	textInput      textinput.Model
	activeVault    string
	activeVaultKey []byte
	tempPassword   string
	newPassword    string
	err            error
	windowWidth    int
	windowHeight   int
}

func initialModel(cfg *Config) model {
	ti := textinput.New()
	ti.Focus()

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.DisableQuitKeybindings()

	m := model{
		config:    cfg,
		textInput: ti,
		list:      l,
	}

	if cfg == nil {
		m.state = stateInitVaultRoot
		m.textInput.Placeholder = "~/.vaulted"
		m.textInput.CharLimit = 156
		m.textInput.Width = 30
	} else {
		m.state = stateNotebookList
		m.list.Title = "Notebooks"
		m.loadNotebooks()
	}

	return m
}

func (m *model) loadNotebooks() {
	if m.config == nil {
		return
	}
	os.MkdirAll(m.config.VaultRoot, 0700)
	entries, err := os.ReadDir(m.config.VaultRoot)
	if err != nil {
		m.err = err
		return
	}

	var items []list.Item
	for _, e := range entries {
		if e.IsDir() && e.Name() != ".archive" {
			items = append(items, item{title: e.Name(), desc: "Notebook (d: delete, a: archive, c: change password)"})
		}
	}
	items = append(items, item{title: "+ Create New Notebook", desc: "Create a new encrypted directory"})

	m.list.SetItems(items)
	m.list.Title = "Notebooks (s: Settings)"
}

func (m *model) loadNotes() {
	if m.config == nil || m.activeVault == "" {
		return
	}
	vaultPath := filepath.Join(m.config.VaultRoot, m.activeVault)
	entries, err := os.ReadDir(vaultPath)
	if err != nil {
		m.err = err
		return
	}

	var items []list.Item
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".enc") {
			items = append(items, item{title: e.Name(), desc: "Encrypted Note"})
		}
	}
	items = append(items, item{title: "+ Create New Note", desc: "Create a new encrypted note"})

	m.list.SetItems(items)
	m.list.Title = "Notes in " + m.activeVault
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth, m.windowHeight = msg.Width, msg.Height
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.err != nil {
			if msg.String() == "enter" || msg.String() == "esc" {
				m.err = nil
			}
			return m, nil
		}
	}

	// State specific handling
	switch m.state {
	case stateInitVaultRoot:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			val := m.textInput.Value()
			if val == "" {
				val = "~/.vaulted"
			}
			if strings.HasPrefix(val, "~/") {
				home, _ := os.UserHomeDir()
				val = filepath.Join(home, val[2:])
			}
			
			editor := os.Getenv("EDITOR")
			if editor == "" {
				if _, err := exec.LookPath("micro"); err == nil {
					editor = "micro"
				} else {
					editor = "nano"
				}
			}

			cfg := &Config{
				VaultRoot: val,
				Editor:    editor,
			}
			if err := saveConfig(cfg); err != nil {
				m.err = err
			} else {
				m.config = cfg
				m.state = stateNotebookList
				m.loadNotebooks()
			}
		}

	case stateNotebookList:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok && m.list.FilterState() != list.Filtering {
			switch keyMsg.String() {
			case "s":
				m.state = stateSettings
				m.list.SetItems([]list.Item{
					item{title: "Change Vault Location", desc: fmt.Sprintf("Current: %s", m.config.VaultRoot)},
					item{title: "Back", desc: "Return to Notebooks"},
				})
				m.list.Title = "Settings"
			case "d":
				selected := m.list.SelectedItem()
				if selected != nil {
					title := selected.(item).title
					if title != "+ Create New Notebook" {
						m.activeVault = title
						m.state = stateDeleteAuth
						m.textInput.Reset()
						m.textInput.Placeholder = "Password to delete " + m.activeVault
						m.textInput.EchoMode = textinput.EchoPassword
						m.textInput.Focus()
					}
				}
			case "c":
				selected := m.list.SelectedItem()
				if selected != nil {
					title := selected.(item).title
					if title != "+ Create New Notebook" {
						m.activeVault = title
						m.state = stateChangePasswordAuth
						m.textInput.Reset()
						m.textInput.Placeholder = "Current password for " + m.activeVault
						m.textInput.EchoMode = textinput.EchoPassword
						m.textInput.Focus()
					}
				}
			case "a":
				selected := m.list.SelectedItem()
				if selected != nil {
					title := selected.(item).title
					if title != "+ Create New Notebook" {
						src := filepath.Join(m.config.VaultRoot, title)
						dstDir := filepath.Join(m.config.VaultRoot, ".archive")
						os.MkdirAll(dstDir, 0700)
						dst := filepath.Join(dstDir, title)
						os.Rename(src, dst)
						m.loadNotebooks()
					}
				}
			case "enter":
				selected := m.list.SelectedItem()
				if selected != nil {
					title := selected.(item).title
					if title == "+ Create New Notebook" {
						m.state = stateNewNotebookPassword
						m.textInput.Reset()
						m.textInput.Placeholder = "New Notebook Name"
						m.textInput.EchoMode = textinput.EchoNormal
						m.textInput.Focus()
					} else {
						m.activeVault = title
						// Check if vault has metadata
						vaultPath := filepath.Join(m.config.VaultRoot, m.activeVault)
						metaPath := filepath.Join(vaultPath, ".vault_meta")
						if _, err := os.Stat(metaPath); os.IsNotExist(err) {
							// Needs setup
							m.state = stateNewNotebookPassword
							m.textInput.Reset()
							m.textInput.Placeholder = "Set Password for " + m.activeVault
							m.textInput.EchoMode = textinput.EchoPassword
							m.textInput.Focus()
						} else {
							// Needs auth
							m.state = stateAuth
							m.textInput.Reset()
							m.textInput.Placeholder = "Password"
							m.textInput.EchoMode = textinput.EchoPassword
							m.textInput.Focus()
						}
					}
				}
			}
		}

	case stateNewNotebookPassword:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			if m.textInput.Placeholder == "New Notebook Name" {
				name := m.textInput.Value()
				if name != "" {
					m.activeVault = name
					vaultPath := filepath.Join(m.config.VaultRoot, m.activeVault)
					os.MkdirAll(vaultPath, 0700)
					m.textInput.Reset()
					m.textInput.Placeholder = "Set Password for " + m.activeVault
					m.textInput.EchoMode = textinput.EchoPassword
					m.textInput.Focus()
				}
			} else {
				password := m.textInput.Value()
				m.tempPassword = password
				m.state = stateNewNotebookPasswordConfirm
				m.textInput.Reset()
				m.textInput.Placeholder = "Confirm Password for " + m.activeVault
				m.textInput.EchoMode = textinput.EchoPassword
				m.textInput.Focus()
			}
		} else if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			m.state = stateNotebookList
			m.activeVault = ""
		}

	case stateNewNotebookPasswordConfirm:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			password := m.textInput.Value()
			if password == m.tempPassword {
				vaultPath := filepath.Join(m.config.VaultRoot, m.activeVault)
				os.MkdirAll(vaultPath, 0700)
				if err := initVault(vaultPath, password); err != nil {
					m.err = err
				} else {
					// Immediately log in
					key, valid, err := loadVault(vaultPath, password)
					if err != nil {
						m.err = err
					} else if valid {
						m.activeVaultKey = key
						m.state = stateNoteList
						m.tempPassword = ""
						m.loadNotes()
					}
				}
			} else {
				m.err = fmt.Errorf("passwords do not match")
				m.state = stateNewNotebookPassword
				m.textInput.Reset()
				m.textInput.Placeholder = "Set Password for " + m.activeVault
				m.textInput.EchoMode = textinput.EchoPassword
				m.textInput.Focus()
			}
		} else if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			m.state = stateNotebookList
			m.activeVault = ""
			m.tempPassword = ""
		}

	case stateAuth:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				password := m.textInput.Value()
				vaultPath := filepath.Join(m.config.VaultRoot, m.activeVault)
				key, valid, err := loadVault(vaultPath, password)
				if err != nil {
					m.err = err
				} else if !valid {
					m.err = fmt.Errorf("invalid password")
					m.textInput.Reset()
				} else {
					m.activeVaultKey = key
					m.state = stateNoteList
					m.loadNotes()
				}
			} else if keyMsg.String() == "esc" {
				m.state = stateNotebookList
				m.activeVault = ""
			}
		}

	case stateNoteList:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "esc" || msg.String() == "backspace" {
				// Clear memory
				for i := range m.activeVaultKey {
					m.activeVaultKey[i] = 0
				}
				m.activeVaultKey = nil
				m.activeVault = ""
				m.state = stateNotebookList
				m.loadNotebooks()
			} else if msg.String() == "enter" {
				selected := m.list.SelectedItem()
				if selected != nil {
					title := selected.(item).title
					if title == "+ Create New Note" {
						m.state = stateNewNoteName
						m.textInput.Reset()
						m.textInput.Placeholder = "Note Name (without .enc)"
						m.textInput.EchoMode = textinput.EchoNormal
						m.textInput.Focus()
					} else {
						notePath := filepath.Join(m.config.VaultRoot, m.activeVault, title)
						return m, openEditor(m.config.Editor, notePath, m.activeVaultKey)
					}
				}
			} else if msg.String() == "n" && m.list.FilterState() != list.Filtering {
				m.state = stateNewNoteName
				m.textInput.Reset()
				m.textInput.Placeholder = "Note Name (without .enc)"
				m.textInput.EchoMode = textinput.EchoNormal
				m.textInput.Focus()
			}

		case editorFinishedMsg:
			if msg.err != nil {
				m.err = msg.err
			}
			m.loadNotes() // Refresh list
		}

	case stateNewNoteName:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				name := m.textInput.Value()
				if name != "" {
					if !strings.HasSuffix(name, ".enc") {
						name += ".enc"
					}
					notePath := filepath.Join(m.config.VaultRoot, m.activeVault, name)
					
					// Change state before opening editor
					m.state = stateNoteList
					return m, openEditor(m.config.Editor, notePath, m.activeVaultKey)
				}
			} else if keyMsg.String() == "esc" {
				m.state = stateNoteList
			}
		}

	case stateSettings:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok && m.list.FilterState() != list.Filtering {
			switch keyMsg.String() {
			case "esc":
				m.state = stateNotebookList
				m.loadNotebooks()
			case "enter":
				selected := m.list.SelectedItem()
				if selected != nil {
					title := selected.(item).title
					if title == "Change Vault Location" {
						m.state = stateSettingVaultRoot
						m.textInput.Reset()
						m.textInput.Placeholder = m.config.VaultRoot
						m.textInput.EchoMode = textinput.EchoNormal
						m.textInput.Focus()
					} else if title == "Back" {
						m.state = stateNotebookList
						m.loadNotebooks()
					}
				}
			}
		}

	case stateSettingVaultRoot:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				val := m.textInput.Value()
				if val != "" {
					if strings.HasPrefix(val, "~/") {
						home, _ := os.UserHomeDir()
						val = filepath.Join(home, val[2:])
					}
					m.config.VaultRoot = val
					if err := saveConfig(m.config); err != nil {
						m.err = err
					} else {
						m.state = stateNotebookList
						m.loadNotebooks()
					}
				}
			} else if keyMsg.String() == "esc" {
				m.state = stateSettings
			}
		}

	case stateDeleteAuth:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				password := m.textInput.Value()
				vaultPath := filepath.Join(m.config.VaultRoot, m.activeVault)
				_, valid, err := loadVault(vaultPath, password)
				if err != nil {
					m.err = err
				} else if !valid {
					m.err = fmt.Errorf("invalid password")
					m.textInput.Reset()
				} else {
					os.RemoveAll(vaultPath)
					m.activeVault = ""
					m.state = stateNotebookList
					m.loadNotebooks()
				}
			} else if keyMsg.String() == "esc" {
				m.activeVault = ""
				m.state = stateNotebookList
			}
		}

	case stateChangePasswordAuth:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				password := m.textInput.Value()
				vaultPath := filepath.Join(m.config.VaultRoot, m.activeVault)
				_, valid, err := loadVault(vaultPath, password)
				if err != nil {
					m.err = err
				} else if !valid {
					m.err = fmt.Errorf("invalid current password")
					m.textInput.Reset()
				} else {
					m.tempPassword = password
					m.state = stateChangePasswordNew
					m.textInput.Reset()
					m.textInput.Placeholder = "New password for " + m.activeVault
					m.textInput.EchoMode = textinput.EchoPassword
					m.textInput.Focus()
				}
			} else if keyMsg.String() == "esc" {
				m.activeVault = ""
				m.state = stateNotebookList
			}
		}

	case stateChangePasswordNew:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				newPassword := m.textInput.Value()
				m.newPassword = newPassword
				m.state = stateChangePasswordConfirm
				m.textInput.Reset()
				m.textInput.Placeholder = "Confirm new password for " + m.activeVault
				m.textInput.EchoMode = textinput.EchoPassword
				m.textInput.Focus()
			} else if keyMsg.String() == "esc" {
				m.tempPassword = ""
				m.activeVault = ""
				m.state = stateNotebookList
			}
		}

	case stateChangePasswordConfirm:
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				confirmPassword := m.textInput.Value()
				if confirmPassword == m.newPassword {
					vaultPath := filepath.Join(m.config.VaultRoot, m.activeVault)
					if err := changeVaultPassword(vaultPath, m.tempPassword, m.newPassword); err != nil {
						m.err = err
					} else {
						m.tempPassword = ""
						m.newPassword = ""
						m.activeVault = ""
						m.state = stateNotebookList
						m.loadNotebooks()
					}
				} else {
					m.err = fmt.Errorf("passwords do not match")
					m.state = stateChangePasswordNew
					m.textInput.Reset()
					m.textInput.Placeholder = "New password for " + m.activeVault
					m.textInput.EchoMode = textinput.EchoPassword
					m.textInput.Focus()
				}
			} else if keyMsg.String() == "esc" {
				m.tempPassword = ""
				m.newPassword = ""
				m.activeVault = ""
				m.state = stateNotebookList
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Error: %v\n\nPress enter to continue.", m.err))
	}

	switch m.state {
	case stateInitVaultRoot:
		return fmt.Sprintf(
			"Welcome to VaultUI!\n\nWhere would you like to store your notebooks?\n\n%s\n\n(Press Enter to confirm)",
			m.textInput.View(),
		)

	case stateNotebookList:
		return lipgloss.NewStyle().Margin(1, 2).Render(m.list.View())

	case stateNewNotebookPassword:
		return fmt.Sprintf(
			"Create Notebook\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.textInput.View(),
		)

	case stateNewNotebookPasswordConfirm:
		return fmt.Sprintf(
			"Confirm Password\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.textInput.View(),
		)

	case stateAuth:
		return fmt.Sprintf(
			"Unlock %s\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.activeVault,
			m.textInput.View(),
		)

	case stateNoteList:
		return lipgloss.NewStyle().Margin(1, 2).Render(m.list.View())

	case stateNewNoteName:
		return fmt.Sprintf(
			"New Note\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.textInput.View(),
		)

	case stateSettings:
		return lipgloss.NewStyle().Margin(1, 2).Render(m.list.View())

	case stateSettingVaultRoot:
		return fmt.Sprintf(
			"Change Vault Location\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.textInput.View(),
		)

	case stateDeleteAuth:
		return fmt.Sprintf(
			"Delete %s\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.activeVault,
			m.textInput.View(),
		)

	case stateChangePasswordAuth:
		return fmt.Sprintf(
			"Change Password for %s\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.activeVault,
			m.textInput.View(),
		)

	case stateChangePasswordNew:
		return fmt.Sprintf(
			"New Password for %s\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.activeVault,
			m.textInput.View(),
		)

	case stateChangePasswordConfirm:
		return fmt.Sprintf(
			"Confirm New Password for %s\n\n%s\n\n(Press Enter to confirm, Esc to go back)",
			m.activeVault,
			m.textInput.View(),
		)
	}

	return ""
}
