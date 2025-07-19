package entities

// This entity represents a scripts, it can be editable

type Script struct {
	Name         string   `json:"name"`
	Placeholders []string `json:"placeholders"`
	Description  string   `json:"description"`
	FilePath     string   `json:"file_path,omitempty"`
	Scope        string   `json:"scope"` // Directory path or "global"
}
