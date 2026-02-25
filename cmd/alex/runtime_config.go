package main

import (
	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
)

func loadRuntimeConfigSnapshot() (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
	envLookup := runtimeEnvLookup()
	overrides, err := loadManagedOverrides(envLookup)
	if err != nil {
		return runtimeconfig.RuntimeConfig{}, runtimeconfig.Metadata{}, err
	}
	return runtimeconfig.Load(
		runtimeconfig.WithEnv(envLookup),
		runtimeconfig.WithOverrides(overrides),
	)
}

func loadManagedOverrides(envLookup runtimeconfig.EnvLookup) (runtimeconfig.Overrides, error) {
	storePath := managedOverridesPath(envLookup)
	store := configadmin.NewFileStore(storePath)
	overrides, err := store.LoadOverrides(cliBaseContext())
	if err != nil {
		return runtimeconfig.Overrides{}, err
	}
	return overrides, nil
}

func saveManagedOverrides(envLookup runtimeconfig.EnvLookup, overrides runtimeconfig.Overrides) error {
	storePath := managedOverridesPath(envLookup)
	store := configadmin.NewFileStore(storePath)
	return store.SaveOverrides(cliBaseContext(), overrides)
}

func runtimeEnvLookup() runtimeconfig.EnvLookup {
	return runtimeconfig.DefaultEnvLookup
}

func managedOverridesPath(envLookup runtimeconfig.EnvLookup) string {
	return configadmin.ResolveStorePath(envLookup)
}
