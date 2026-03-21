package entities

type Script struct {
	ID                         string `json:"id"`
	Name                       string `json:"name"`
	Description                string `json:"description"`
	FilePath                   string `json:"file_path,omitempty"`
	Scope                      string `json:"scope"`
	PlaceholderStartDemarcator string `json:"placeholder_start_demarcator,omitempty"`
	PlaceholderEndDemarcator   string `json:"placeholder_end_demarcator,omitempty"`
	Archived                   bool   `json:"archived,omitempty"`
}
