package modelcache

type JujuModel jujuModel

func (m ModelLookup) Name() string {
	return m.name
}

func (m ModelLookup) Owner() string {
	return m.owner
}

func (m *Cache) Length() int {
	return len(m.modelMap)
}

func (m *Cache) LengthByName(name string) int {
	models, ok := m.modelMap[name]
	if !ok {
		return 0
	}
	return len(models.models)
}
