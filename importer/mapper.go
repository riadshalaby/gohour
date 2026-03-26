package importer

import (
	"fmt"
	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/worklog"
)

type Mapper interface {
	Name() string
	Map(record Record, cfg config.Config, sourceFormat, sourceFile string) (*worklog.Entry, bool, error)
}

func SupportedMapperNames() []string {
	return []string{"epm", "generic", "atwork"}
}

func MapperByName(name string) (Mapper, error) {
	switch normalizeHeader(name) {
	case "epm":
		return &EPMMapper{}, nil
	case "generic":
		return &GenericMapper{}, nil
	case "atwork":
		return &ATWorkMapper{}, nil
	default:
		return nil, fmt.Errorf("unsupported mapper: %s", name)
	}
}
