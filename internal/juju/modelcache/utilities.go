// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package modelcache

import (
	"fmt"
	"slices"

	"github.com/juju/juju/core/model"
)

type jujuModel struct {
	Name      string
	Owner     string
	UUID      string
	ModelType model.ModelType
}

func (j jujuModel) String() string {
	return fmt.Sprintf("uuid(%s) type(%s)", j.UUID, j.ModelType.String())
}

type modelSlice struct {
	models []jujuModel
}

func (m *modelSlice) removeByIndex(index int) {
	m.models = slices.Delete(m.models, index, index+1)
}

func (m *modelSlice) addModel(modelInfo jujuModel) {
	exists := slices.ContainsFunc(m.models, func(m jujuModel) bool {
		return m.UUID == modelInfo.UUID
	})
	if exists {
		return
	}
	m.models = append(m.models, modelInfo)
}
