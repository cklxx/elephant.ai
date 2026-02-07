Practice: Add server-side e2e tests (`httptest.NewServer`) for critical route availability, especially auth entrypoints and dev observability APIs.
Impact: Prevents false “service started but unusable” regressions by catching route wiring/auth-mode issues in CI before release.
