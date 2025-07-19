package entities

// This entity represents a scripts, it can be editable

type Script struct {
	Name         string   `json:"name"`
	Command      string   `json:"command"`
	Placeholders []string `json:"placeholders"`
	Description  string   `json:"description"`
	FilePath     string   `json:"file_path,omitempty"`
}
