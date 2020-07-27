package upload

import (
	"strings"

	"github.com/c2h5oh/datasize"
	"github.com/pkg/errors"
)

// MaxSize contains a list of size overrides. A zero-value instance is a valid
// instance.
type MaxSize struct {
	vals map[string]datasize.ByteSize
	keys []string
}

func (c *MaxSize) UnmarshalTOML(v interface{}) error {
	mp, ok := v.(map[string]interface{})
	if !ok {
		return errors.New("invalid TOML type; should be key-value map")
	}

	if c.vals == nil {
		c.vals = make(map[string]datasize.ByteSize, len(mp))
	}

	for k, v := range mp {
		sr, ok := v.(string)
		if !ok {
			return errors.New("invalid TOML type; should be string")
		}

		var b datasize.ByteSize
		if err := b.UnmarshalText([]byte(sr)); err != nil {
			return err
		}

		c.vals[k] = b
		c.keys = append(c.keys, k)
	}

	return nil
}

func (c MaxSize) SizeLimit(ctype string) (bytes datasize.ByteSize) {
	for _, cfgtype := range c.keys {
		if strings.Contains(ctype, cfgtype) {
			bytes = c.vals[cfgtype]
		}
	}
	return
}
