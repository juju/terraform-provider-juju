package modelcache_test

import (
	"testing"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/core/model"
	"github.com/juju/terraform-provider-juju/internal/juju/modelcache"
	"github.com/stretchr/testify/assert"
)

func TestModelLookup(t *testing.T) {
	lookup := modelcache.NewModelLookup("foo")
	assert.Equal(t, lookup.String(), "foo")
	assert.Equal(t, lookup.Name(), "foo")
	assert.Equal(t, lookup.Owner(), "")

	lookup = modelcache.NewModelLookup("owner/foo")
	assert.Equal(t, lookup.String(), "owner/foo")
	assert.Equal(t, lookup.Name(), "foo")
	assert.Equal(t, lookup.Owner(), "owner")
}

func TestFillCache(t *testing.T) {
	testCases := []struct {
		desc                  string
		modelSummaries        []base.UserModelSummary
		expectedMapLength     int
		expectedModelsPerName map[string]int
	}{
		{
			desc: "Basic fill",
			modelSummaries: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1"},
				{Name: "wolf", UUID: "2", Owner: "user-2"},
				{Name: "bird", UUID: "3", Owner: "user-3"},
			},
			expectedMapLength: 3,
			expectedModelsPerName: map[string]int{
				"fox":  1,
				"wolf": 1,
				"bird": 1,
			},
		},
		{
			desc: "Fill with duplicates",
			modelSummaries: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1"},
				{Name: "fox", UUID: "1", Owner: "user-1"},
				{Name: "fox", UUID: "1", Owner: "user-1"},
			},
			expectedMapLength: 1,
			expectedModelsPerName: map[string]int{
				"fox": 1,
			},
		},
		{
			desc: "Multiple models with same name but different owner",
			modelSummaries: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1"},
				{Name: "fox", UUID: "2", Owner: "user-2"},
				{Name: "fox", UUID: "3", Owner: "user-3"},
			},
			expectedMapLength: 1,
			expectedModelsPerName: map[string]int{
				"fox": 3,
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			modelCache := modelcache.NewModelCache()
			modelCache.FillCache(tC.modelSummaries)
			assert.Equal(t, tC.expectedMapLength, modelCache.Length())
			for name, expectedModels := range tC.expectedModelsPerName {
				assert.Equal(t, expectedModels, modelCache.LengthByName(name))
			}
		})
	}
}

func TestLookup(t *testing.T) {
	testCases := []struct {
		desc          string
		lookup        modelcache.ModelLookup
		cacheValues   []base.UserModelSummary
		expectedModel modelcache.JujuModel
		expectedError string
	}{
		{
			desc:   "lookup by name only",
			lookup: modelcache.NewModelLookup("fox"),
			cacheValues: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1", Type: "myType"},
				{Name: "wolf", UUID: "2", Owner: "user-2"},
				{Name: "bird", UUID: "3", Owner: "user-3"},
			},
			expectedModel: modelcache.JujuModel{
				Name:      "fox",
				Owner:     "user-1",
				UUID:      "1",
				ModelType: "myType",
			},
		},
		{
			desc:   "lookup by name and owner",
			lookup: modelcache.NewModelLookup("user-1/fox"),
			cacheValues: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1", Type: "myType"},
				{Name: "wolf", UUID: "2", Owner: "user-2"},
				{Name: "bird", UUID: "3", Owner: "user-3"},
			},
			expectedModel: modelcache.JujuModel{
				Name:      "fox",
				Owner:     "user-1",
				UUID:      "1",
				ModelType: "myType",
			},
		},
		{
			desc:   "lookup by name when multiple models have the same name",
			lookup: modelcache.NewModelLookup("fox"),
			cacheValues: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1"},
				{Name: "fox", UUID: "2", Owner: "user-2"},
				{Name: "bird", UUID: "3", Owner: "user-3"},
			},
			expectedError: `multiple models with name "fox" found, please specify a model owner`,
		},
		{
			desc:   "lookup by name and owner when multiple models have the same name",
			lookup: modelcache.NewModelLookup("user-2/fox"),
			cacheValues: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1"},
				{Name: "fox", UUID: "2", Owner: "user-2", Type: "myType"},
				{Name: "fox", UUID: "3", Owner: "user-3"},
			},
			expectedModel: modelcache.JujuModel{
				Name:      "fox",
				Owner:     "user-2",
				UUID:      "2",
				ModelType: "myType",
			},
		},
		{
			desc:          "lookup with no values",
			lookup:        modelcache.NewModelLookup("fox"),
			cacheValues:   []base.UserModelSummary{},
			expectedError: "not found",
		},
		{
			desc:   "lookup model that doesn't exist",
			lookup: modelcache.NewModelLookup("whale"),
			cacheValues: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1"},
				{Name: "wolf", UUID: "2", Owner: "user-2"},
				{Name: "bird", UUID: "3", Owner: "user-3"},
			},
			expectedError: "not found",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			cache := modelcache.NewModelCache()
			cache.FillCache(tC.cacheValues)
			model, err := cache.Lookup(tC.lookup)
			if tC.expectedError != "" {
				assert.ErrorContains(t, err, tC.expectedError)
			} else {
				assert.EqualValues(t, model, tC.expectedModel)
			}
		})
	}
}

func TestRemoveModel2(t *testing.T) {
	testCases := []struct {
		desc           string
		uuidToRemove   string
		initialvalues  []base.UserModelSummary
		checkModelName string
		expectedLens   int
	}{
		{
			desc:         "remove model",
			uuidToRemove: "2",
			initialvalues: []base.UserModelSummary{
				{Name: "fox", UUID: "1", Owner: "user-1"},
				{Name: "wolf", UUID: "2", Owner: "user-2"},
				{Name: "bird", UUID: "3", Owner: "user-3"},
			},
			checkModelName: "wolf",
			expectedLens:   0,
		},
		{
			desc:         "remove model when multiple of the same name exist",
			uuidToRemove: "2",
			initialvalues: []base.UserModelSummary{
				{Name: "wolf", UUID: "1", Owner: "user-1"},
				{Name: "wolf", UUID: "2", Owner: "user-2"},
				{Name: "wolf", UUID: "3", Owner: "user-3"},
			},
			checkModelName: "wolf",
			expectedLens:   2,
		},
		{
			desc:         "remove model that doesn't exist",
			uuidToRemove: "10",
			initialvalues: []base.UserModelSummary{
				{Name: "wolf", UUID: "1", Owner: "user-1"},
				{Name: "wolf", UUID: "2", Owner: "user-2"},
				{Name: "wolf", UUID: "3", Owner: "user-3"},
			},
			checkModelName: "wolf",
			expectedLens:   3,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			cache := modelcache.NewModelCache()
			cache.FillCache(tC.initialvalues)
			cache.RemoveModel(tC.uuidToRemove)
			assert.Equal(t, tC.expectedLens, cache.LengthByName(tC.checkModelName))
		})
	}
}

func TestAddModel(t *testing.T) {
	type addModelParams struct {
		name  string
		uuid  string
		mtype string
		owner string
	}
	testCases := []struct {
		desc          string
		initialvalues []base.UserModelSummary
		params        []addModelParams
		expectedLens  map[string]int
	}{
		{
			desc: "add 1 model",
			params: []addModelParams{
				{name: "wolf", uuid: "1", mtype: "myType", owner: "user-1"},
			},
			expectedLens: map[string]int{"wolf": 1},
		},
		{
			desc: "add multiple models",
			params: []addModelParams{
				{name: "wolf", uuid: "1", mtype: "myType", owner: "user-1"},
				{name: "wolf", uuid: "2", mtype: "myType", owner: "user-2"},
				{name: "bird", uuid: "3", mtype: "myType", owner: "user-2"},
			},
			expectedLens: map[string]int{"wolf": 2, "bird": 1},
		},
		{
			desc: "add 2 models with the same UUID",
			params: []addModelParams{
				{name: "wolf", uuid: "1", mtype: "myType", owner: "user-1"},
				{name: "wolf-1", uuid: "1", mtype: "myType", owner: "user-2"},
			},
			expectedLens: map[string]int{"wolf": 1, "wolf-1": 0},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			cache := modelcache.NewModelCache()
			if tC.initialvalues != nil {
				cache.FillCache(tC.initialvalues)
			}
			for _, param := range tC.params {
				cache.AddModel(param.name, param.owner, param.uuid, model.ModelType(param.mtype))
			}
			for name, expectedLen := range tC.expectedLens {
				assert.Equal(t, expectedLen, cache.LengthByName(name))
			}
		})
	}
}
