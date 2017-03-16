package shells

type archivingOptions struct {
	Untracked bool     `json:"untracked"`
	Paths     []string `json:"paths"`
	Name      string   `json:"name"`
	Key       string   `json:"key"`
}

type shellOptions struct {
	Cache     *archivingOptions `json:"cache"`
	Artifacts *archivingOptions `json:"artifacts"`
}
