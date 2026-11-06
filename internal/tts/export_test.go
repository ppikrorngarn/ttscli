package tts

// registerForTest adds spec to the registry for the duration of a test and
// returns a function that removes it again.
func registerForTest(spec Spec) func() {
	registry[spec.Name] = spec
	return func() { delete(registry, spec.Name) }
}

// setPlannedForTest replaces the planned-provider list for the duration of a
// test and returns a function that restores it.
func setPlannedForTest(names []string) func() {
	saved := plannedProviders
	plannedProviders = names
	return func() { plannedProviders = saved }
}
