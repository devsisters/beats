package docker

var defaultConfig = config{
	Containers: containers{
		IDs:           []string{},
		Path:          "/var/lib/docker/containers",
		Stream:        "all",
		ConcatPartial: false,
	},
}

type config struct {
	Containers containers `config:"containers"`
}

type containers struct {
	IDs  []string `config:"ids"`
	Path string   `config:"path"`

	// Stream can be all,stdout or stderr
	Stream string `config:"stream"`
	// if true, concatnate partial logs (splited due to long size exceeds 16k)
	ConcatPartial bool `config:"concat_partial"`
}
