package entities

type Script struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	FilePath    string `json:"file_path,omitempty"`
	Scope       string `json:"scope"`
}
