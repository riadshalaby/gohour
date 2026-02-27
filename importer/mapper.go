package importer

import (
	"fmt"
	"gohour/config"
	"gohour/worklog"
)

type Mapper interface {
	Name() string
	Map(record Record, cfg config.Config, sourceFormat, sourceFile string) (*worklog.Entry, bool, error)
}

func SupportedMapperNames() []string {
	return []string{"epm", "generic"}
}

func MapperByName(name string) (Mapper, error) {
	switch normalizeHeader(name) {
	case "epm":
		return &EPMMapper{}, nil
	case "generic":
		return &GenericMapper{}, nil
	default:
		return nil, fmt.Errorf("unsupported mapper: %s", name)
	}
}
