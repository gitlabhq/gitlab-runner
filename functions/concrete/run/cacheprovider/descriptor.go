package cacheprovider

type Descriptor struct {
	URL        string              `json:"url,omitempty"`
	Env        map[string]string   `json:"env,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	GoCloudURL bool                `json:"go_cloud_url,omitempty"`
	// HeadURL is the pre-signed HEAD URL used by cache-archiver (--check-url)
	// to skip uploading when the object already exists. Only populated for
	// upload descriptors and only when the adapter supports HEAD.
	HeadURL string `json:"head_url,omitempty"`
}
