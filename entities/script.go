package entities

type Script struct {
	ID                         string `json:"id"`
	Name                       string `json:"name"`
	Description                string `json:"description"`
	FilePath                   string `json:"file_path,omitempty"`
	Scope                      string `json:"scope"`
	Archived bool `json:"archived,omitempty"`
	OriginalScope              string `json:"-"`
}
