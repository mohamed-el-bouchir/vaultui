package main

import (
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

type editorFinishedMsg struct{ err error }

func openEditor(editorCmd, notePath string, key []byte) tea.Cmd {
	// 1. Read existing content and decrypt (if it exists)
	var content []byte
	if _, err := os.Stat(notePath); err == nil {
		encryptedContent, err := os.ReadFile(notePath)
		if err == nil && len(encryptedContent) > 0 {
			content, err = decrypt(encryptedContent, key)
			if err != nil {
				return func() tea.Msg { return editorFinishedMsg{err} }
			}
		}
	}

	// 2. Create temp file
	tmpFile, err := os.CreateTemp("", "vaultui-note-*.md")
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err} }
	}
	tmpPath := tmpFile.Name()

	if len(content) > 0 {
		if _, err := tmpFile.Write(content); err != nil {
			tmpFile.Close()
			return func() tea.Msg { return editorFinishedMsg{err} }
		}
	}
	tmpFile.Close()

	// 3. Spawning the editor using tea.ExecProcess
	c := exec.Command(editorCmd, tmpPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		defer os.Remove(tmpPath) // Make sure to delete the temp file

		if err != nil {
			return editorFinishedMsg{err}
		}

		// 4. Read back the modified temp file
		newContent, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return editorFinishedMsg{readErr}
		}

		// 5. Encrypt and save back
		encryptedContent, encErr := encrypt(newContent, key)
		if encErr != nil {
			return editorFinishedMsg{encErr}
		}

		if writeErr := os.WriteFile(notePath, encryptedContent, 0600); writeErr != nil {
			return editorFinishedMsg{writeErr}
		}

		return editorFinishedMsg{nil}
	})
}
