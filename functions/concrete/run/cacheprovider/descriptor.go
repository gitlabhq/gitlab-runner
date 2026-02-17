package cacheprovider

type Descriptor struct {
	URL        string              `json:"url,omitempty"`
	Env        map[string]string   `json:"env,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	GoCloudURL bool                `json:"go_cloud_url,omitempty"`
}
