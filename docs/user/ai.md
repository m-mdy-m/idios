# AI providers and reading assistants

IDIOS can use local model servers and configured remote providers. You choose the provider, model, and privacy policy from **Settings → AI**.

## 1. Understand the parts

- A **provider** describes where models are discovered and how requests are sent.
- A **model** is a selectable model exposed by a provider.
- An **agent** is a named role with instructions, a provider/model selection, and optional skills.
- A **skill** is a reusable instruction package that helps an agent perform one kind of task.

A model being visible in the catalogue does not mean it is currently reachable or permitted for inference.

## 2. Add or configure a provider

Open **Settings → AI → Providers** and configure the provider's endpoint and authentication fields. Keep API keys private. Do not paste secrets into issue reports, screenshots, or source files.

IDIOS includes an Ollama Local provider and supports configured OpenAI-compatible endpoints such as local routers or hosted gateways. Extensions can register additional Go provider adapters; adding an adapter requires rebuilding the IDIOS executable.

## 3. Refresh one model catalogue

1. Open **Settings → AI → Models**.
2. Press **Refresh catalogue**.
3. Choose exactly one configured provider in the provider picker.
4. Confirm the refresh.
5. Review the result for that provider.

IDIOS does not refresh every provider when you open the picker. A refresh request must include a `provider_id`; the server queries only that provider. Cached models from unselected providers remain unchanged.

If Ollama Local is selected, IDIOS may also check the availability of models returned by that Ollama instance. It does not run Ollama probes for models from unrelated providers.

## 4. Choose a privacy policy

- **Offline-only:** remote providers are not contacted for discovery or inference. The server enforces this rule even if a client sends a request directly.
- **Local-first:** local inference is preferred. Remote catalogue metadata may be discovered only when you explicitly select a remote provider; remote inference remains blocked unless the cloud fallback setting is enabled.
- **Cloud-first:** remote providers may be used according to the provider and model configuration.

Read the exact labels and warnings shown in your current Settings page before changing a privacy option. Do not assume that a remote model shown in the catalogue is allowed to run under the current policy.

## 5. Use AI in Shelf

Shelf uses MIMI as its primary reading companion for page-margin notes and conversational tutoring. It also includes contextual translation, selected reading skills, and optional evidence-led agent discussions. The context may include the book title, summary, subject tags, current page, saved highlights, and notes. The selected passage remains the primary source for passage-specific operations.

A generated response is only a suggestion. For notes and highlights, use the explicit save action and wait for the saved or failed result from IDIOS. Never treat model text alone as proof that a database write happened.

## 6. Structured output and custom adapters

The runtime defines `text`, `json_object`, and `json_schema` response formats. An adapter must translate a format only when its backend supports it; callers must still validate returned JSON before trusting it.

For implementation details, see [AI and Shelf architecture](../arch/ai-and-shelf.md), [provider refresh contract](../reference/provider-refresh.md), and [backend development](../develop/backend.md).
