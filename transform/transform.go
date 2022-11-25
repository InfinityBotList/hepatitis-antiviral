package transform

import (
	"hepatitis-antiviral/cli"
	"strings"

	"github.com/google/uuid"
)

// Defines a transform converting a comma seperated string to a slice of strings, replaces “tolist“
func ToList(tr cli.TransformRow) any {
	res := tr.CurrentValue
	if resCast, ok := res.(string); ok {
		res = strings.Split(strings.ReplaceAll(resCast, " ", ""), ",")
		return res
	} else {
		if res == nil {
			return []string{}
		}
		return res
	}
}

// Defines a default value transformer for a field, replaces “defaultfunc“
func DefaultTransform(f cli.TransformFunc) cli.TransformFunc {
	return func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			return f(tr)
		}
		return tr.CurrentValue
	}
}

// Defines a transform converting a string to a uuid, replaces "uuidgen",
func UUID(tr cli.TransformRow) any {
	uuid := uuid.New()
	return uuid.String()
}

// Defines a transform converting a string to a uuid as a default function, replaces "defaultfunc:uuidgen"
func UUIDDefault(tr cli.TransformRow) any {
	return DefaultTransform(UUID)(tr)
}

// Defines a transform that only applies when the current value is not null
func TransformIfExists(f cli.TransformFunc) cli.TransformFunc {
	return func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			return nil
		}
		return f(tr)
	}
}
