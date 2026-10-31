# IDIOS documentation

IDIOS is a local workspace for learning and development. You can use it from the browser or from a terminal. These pages explain what it does, how to use it, and how to change the source code.

## Understand the product

| If you want to… | Read |
|---|---|
| Learn what IDIOS is | [Product overview](product/overview.md) |
| See what every module does | [Feature map](product/features.md) |
| Understand IDIOS terminology | [Terms and concepts](product/terms.md) |
| Learn where data stays and when AI providers are contacted | [Privacy and local data](product/privacy.md) |

## Start as a user

| If you want to… | Read |
|---|---|
| Install IDIOS and open the workspace | [Getting started](user/getting-started.md) |
| Install IDIOS as a PWA or build Android/iOS clients | [Mobile and PWA](user/mobile.md) |
| Understand Repositories, Issues, Projects, and Pull Requests | [Workspace guide](user/workspace.md) |
| Add books, filter your library, and read PDFs | [Shelf guide](user/shelf.md) |
| Add AI providers and refresh one provider's models | [AI guide](user/ai.md) |
| Create plans, reminders, and scheduled jobs | [Plans and jobs](user/plans-and-jobs.md) |
| Change settings and back up your data | [Settings and data](user/settings-and-data.md) |
| Use IDIOS from a terminal | [CLI guide](user/cli.md) |
| Fix a common setup or runtime problem | [Troubleshooting](user/troubleshooting.md) |

## Start as a developer

| If you want to… | Read |
|---|---|
| Set up the source tree | [Developer guide](develop/README.md) |
| Build and run IDIOS | [Build and run](develop/build-and-run.md) |
| Run the tests and understand the checks | [Testing](develop/testing.md) |
| Add a feature without coupling unrelated modules | [Adding a feature](develop/adding-a-feature.md) |
| Work on the Web client | [Web development](develop/web.md) |
| Work on Go services and HTTP handlers | [Backend development](develop/backend.md) |
| Change SQLite data safely | [Storage and migrations](develop/storage.md) |
| Run the shared local and GitHub pipeline | [Pipeline](develop/pipeline.md) |
| Prepare a release | [Releasing](develop/releasing.md) |

## Architecture and reference

- [Architecture index](arch/README.md)
- [Architecture overview](arch/overview.md)
- [Feature map](product/features.md)
- [Service boundaries](arch/service-boundaries.md)
- [Storage model](arch/storage.md)
- [AI and Shelf design](arch/ai-and-shelf.md)
- [HTTP API](arch/http-api.md)
- [CLI reference](reference/cli.md)
- [Configuration reference](reference/configuration.md)
- [Routes reference](reference/routes.md)
- [AI provider refresh contract](reference/provider-refresh.md)

## Documentation rules

- The application source is authoritative when behavior changes. Update the matching guide in the same change.
- Examples are written for a fresh installation unless a page says otherwise.
- A check is called **passed** only when it has actually been run. Developer pages list commands and mark environment-dependent checks as not run when applicable.
- “Repository”, “Pull Request”, and “Commit” are IDIOS workspace records unless the page explicitly discusses the GitHub source repository.
