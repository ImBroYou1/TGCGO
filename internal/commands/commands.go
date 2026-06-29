package commands

import (
	"encoding/json"
	"os"
)

type Command struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Bash        string `json:"bash"`
	Created     string `json:"created"`
}

func getPath() string {
	return "data/custom_commands.json"
}

func Load() []Command {
	data, err := os.ReadFile(getPath())
	if err != nil {
		return []Command{}
	}

	var result struct {
		Commands []Command `json:"commands"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return []Command{}
	}

	return result.Commands
}

func Save(cmds []Command) {
	os.MkdirAll("data", 0755)

	result := struct {
		Commands []Command `json:"commands"`
	}{Commands: cmds}

	data, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(getPath(), data, 0644)
}
