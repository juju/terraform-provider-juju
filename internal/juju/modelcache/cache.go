package modelcache

import (
	"strings"
	"sync"

	"github.com/juju/errors"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/core/model"
)

// ModelLookup contains request parameters to gather info on a model.
type ModelLookup struct {
	name  string
	owner string
}

// String returns a string representation of a ModelLookup object.
func (m ModelLookup) String() string {
	if m.owner == "" {
		return m.name
	}
	return m.owner + "/" + m.name
}

// NewModelLookup returns a ModelLookup object from a model name.
// It is expected that the model name is either fully specified
// as <model-owner>/<model-name> or just <model-name>.
// Excluding the model owner will influence the behavior of the Lookup function.
func NewModelLookup(modelName string) ModelLookup {
	res := strings.Split(modelName, "/")
	if len(res) == 1 {
		return ModelLookup{
			name: res[0],
		}
	}
	return ModelLookup{
		name:  res[1],
		owner: res[0],
	}
}

// Cache is a model cache that can be used to query for models by UUID or model owner and name.
type Cache struct {
	modelsMu sync.Mutex
	// modelMap is a map from model names to a modelSlice object.
	// The key represents a model's name and the modelSlice contains
	// all the models with that name but with unique owners.
	modelMap map[string]modelSlice
}

// NewModelCache returns a new cache object.
func NewModelCache() Cache {
	return Cache{modelMap: make(map[string]modelSlice)}
}

// FillCache populates the cache based on the supplied modelSummaries.
func (m *Cache) FillCache(modelSummaries []base.UserModelSummary) {
	for _, modelSummary := range modelSummaries {
		modelInfo := jujuModel{
			Name:      modelSummary.Name,
			Owner:     modelSummary.Owner,
			UUID:      modelSummary.UUID,
			ModelType: modelSummary.Type,
		}
		models := m.modelMap[modelSummary.Name]
		models.addModel(modelInfo)
		m.modelMap[modelSummary.Name] = models
	}
}

// Lookup retrieves model information based on a ModelLookup object.
// In the ModelLookup object, the owner can be empty but if multiple models
// with the same name are found then a NotAssigned error will be returned.
// If a model is not found then a NotFound error will be returned.
func (m *Cache) Lookup(lookup ModelLookup) (jujuModel, error) {
	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()
	models, ok := m.modelMap[lookup.name]
	if !ok {
		return jujuModel{}, errors.NotFoundf("model %q", lookup)
	}
	if len(models.models) == 1 && lookup.owner == "" {
		return models.models[0], nil
	}
	if len(models.models) > 1 && lookup.owner == "" {
		return jujuModel{}, errors.NotAssignedf("multiple models with name %q found, please specify a model owner", lookup.name)
	}
	for _, model := range models.models {
		if model.Owner == lookup.owner {
			return model, nil
		}
	}
	return jujuModel{}, errors.NotFoundf("model %q", lookup)
}

type lookupByUUIDResult struct {
	modelSlice modelSlice
	index      int
	name       string
}

func (m *Cache) lookupByUUID(uuid string) (lookupByUUIDResult, bool) {
	for _, models := range m.modelMap {
		for i, model := range models.models {
			if model.UUID == uuid {
				return lookupByUUIDResult{
					modelSlice: models,
					index:      i,
					name:       model.Name,
				}, true
			}
		}
	}
	return lookupByUUIDResult{}, false
}

// RemoveModel removes a model from the cache based on UUID.
func (m *Cache) RemoveModel(uuid string) {
	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()
	res, ok := m.lookupByUUID(uuid)
	if !ok {
		return
	}
	res.modelSlice.removeByIndex(res.index)
	m.modelMap[res.name] = res.modelSlice
}

// AddModel adds a model to the cache.
func (m *Cache) AddModel(modelName, modelOwner, modelUUID string, modelType model.ModelType) {
	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()
	_, ok := m.lookupByUUID(modelUUID)
	if ok {
		return
	}
	models := m.modelMap[modelName]
	models.addModel(jujuModel{
		Name:      modelName,
		Owner:     modelOwner,
		UUID:      modelUUID,
		ModelType: modelType,
	})
	m.modelMap[modelName] = models
}
