# Provider-scoped model refresh contract

Model discovery is an explicit operation against one configured provider.

## HTTP request

`POST /api/ai/models/refresh` requires a JSON body with `provider_id`:

```json
{
  "provider_id": "ollama-local"
}
```

An empty, unknown, or disabled provider ID is rejected. Under Offline-only, a remote provider is rejected by the server even if a client bypasses the UI.

## Runtime behavior

`Manager.RefreshProviderModels(ctx, providerID)` discovers models only from the selected provider. On success it updates that provider's catalogue. On failure the last-known catalogue is preserved. Unselected provider catalogues are not contacted or replaced.

Only the selected Ollama provider runs Ollama-specific model availability probes. Other providers return their model list without triggering Ollama probes.

The Shelf reader uses cached model data on open. Its manual refresh is scoped to the selected model's provider when that provider is unambiguous; otherwise the user is sent to Settings → AI to choose a provider.

## Privacy behavior

- **Offline-only:** no remote catalogue or inference request is sent.
- **Local-first:** a user may explicitly refresh remote catalogue metadata, but remote inference remains blocked unless cloud fallback is enabled.
- **Cloud-first:** provider use follows the configured policy and model selection.

The browser picker is a user experience. The server-side provider ID and privacy checks are the enforcement boundary.
