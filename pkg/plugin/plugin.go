package plugin

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

type Container struct {
	Plugin Plugin
	schema *gojsonschema.Schema
}

func NewContainer(newPlugin NewFunc) (Container, error) {
	plugin, err := newPlugin()
	if err != nil {
		return Container{}, errors.Wrap(err, "failed to instantiate plugin")
	}
	description := plugin.Describe()
	var schema *gojsonschema.Schema
	if description.SpecSchema != nil {
		schema, err = gojsonschema.NewSchema(gojsonschema.NewBytesLoader(description.SpecSchema))
		if err != nil {
			return Container{}, errors.Wrapf(err, "can't use plugin %q due to invalid schema", description.Name)
		}
	}

	return Container{
		Plugin: plugin,
		schema: schema,
	}, nil
}

func (pc *Container) ValidateSpec(pluginSpec map[string]interface{}) error {
	if pc.schema == nil {
		return nil
	}

	validationResult, err := pc.schema.Validate(gojsonschema.NewGoLoader(pluginSpec))
	if err != nil {
		return errors.Wrap(err, "error validating plugin spec")
	}

	if !validationResult.Valid() {
		validationErrors := validationResult.Errors()
		msgs := make([]string, 0, len(validationErrors))

		for _, validationErr := range validationErrors {
			msgs = append(msgs, validationErr.String())
		}

		return errors.Errorf("spec failed validation against schema: %s",
			strings.Join(msgs, ", "))
	}

	return nil
}
