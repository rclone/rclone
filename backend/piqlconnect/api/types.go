package api

type Package struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}
