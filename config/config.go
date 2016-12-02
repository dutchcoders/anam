package config

import (
	"reflect"

	"github.com/minio/cli"
)

type Config struct {
	UseTLS bool `flag:"tls"`

	Port       int `flag:"port"`
	NumThreads int `flag:"threads"`

	Interface string `flag:"interface"`

	Timeout        int    `flag:"timeout"`
	UserAgent      string `flag:"user-agent"`
	EnableProfiler bool   `flag:"profiler"`

	Resolvers string `flag:"resolvers"`
	Prefixes  string `flag:"prefixes"`

	Paths []string
}

func LoadFromContext(c *cli.Context) *Config {
	config := &Config{}

	v2 := reflect.ValueOf(config).Elem()
	v := v2.Type()

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)

		tag := f.Tag.Get("flag")
		if tag == "" {
			continue
		}

		dest := v2.FieldByName(f.Name)

		switch f.Type.Kind() {
		case reflect.Bool:
			dest.SetBool(c.GlobalBool(tag))
		case reflect.Int:
			dest.SetInt(int64(c.GlobalInt(tag)))
		case reflect.String:
			dest.SetString(c.GlobalString(tag))
		}
	}
	return config
}
